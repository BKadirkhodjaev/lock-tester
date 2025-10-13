package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/BKadirkhodjaev/lock-tester/client"
	"github.com/google/uuid"
)

const (
	// System Properties
	URI      string = "http://localhost:8000"
	Tenant   string = "diku"
	Username string = "diku_admin"
	Password string = "admin"

	// FOLIO Entity IDs
	VendorId       string = "e0fb5df2-cdf1-11e8-a8d5-f2801f1b9fd1"
	BatchGroupId   string = "cd592659-77aa-4eb3-ac34-c9a4657bb20f"
	FiscalYearId   string = "43aba0df-70e4-435e-8463-d8f730d19e0b"
	ExpenseClassId string = "1bcc3247-99bf-4dca-9b0f-7bc51a2998c2"

	// Fund IDs
	FundId  string = "7fbd5d84-62d1-44c6-9c45-6cb173998bbd"
	FundId2 string = "55f48dc6-efa7-4cfe-bc7c-4786efe493e3"

	// Budget IDs
	BudgetId  string = "076e9607-2181-42a4-94f7-ed379af0f810"
	BudgetId2 string = "de6761f6-1f64-4b5d-a5bc-98862c498c21"

	// Fund Codes
	FundCode1 string = "AFRICAHIST"
	FundCode2 string = "ASIAHIST"

	// Financial Values
	AllocationAmount  float64 = 10000.0
	AdjustmentValue   float64 = 10.0
	SubTotal          float64 = 10.0
	DistributionValue int     = 100

	// Transaction & Business Logic
	TransactionType  string = "Allocation"
	Currency         string = "USD"
	Source           string = "User"
	InvoiceStatus    string = "Open"
	DistributionType string = "percentage"
	AdjustmentType   string = "Amount"
	RelationToTotal  string = "In addition to"
	Prorate          string = "Not prorated"
	Quantity         string = "1"

	// External System Codes
	AccountingCode string = "G64758-74834"
	PaymentMethod  string = "Credit Card"

	// Descriptions & Text
	InvoiceLineDescription string = "Test invoice line"
)

var (
	threadCount int
	enableDebug bool
)

type application struct {
	logger *slog.Logger
	client *client.RequestClient
}

func main() {
	start := time.Now()

	flag.IntVar(&threadCount, "threads", 50, "Persistent HTTP thread count")
	flag.BoolVar(&enableDebug, "debug", false, "Enable debug output of HTTP request and response")
	flag.Parse()

	logLevel := slog.LevelInfo
	if enableDebug {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true,
	}))
	slog.SetDefault(logger)

	a := &application{logger: logger, client: &client.RequestClient{
		Logger:       logger,
		URI:          URI,
		Tenant:       Tenant,
		RetryMax:     30,
		RetryWaitMax: 500 * time.Second,
	}}

	a.logger.Info("Started")

	token := a.getAccessToken()
	headers := a.client.CreateHeadersWithToken(token)

	a.recalculateBudgets(headers)
	a.createBudgetAllocations(headers)

	invoiceIdCh := make(chan string, threadCount)
	var invoiceCreationWg sync.WaitGroup
	invoiceCreationWg.Add(threadCount)
	for range threadCount {
		go a.createInvoiceAndInvoiceLines(headers, invoiceIdCh, &invoiceCreationWg)
	}

	go func() {
		invoiceCreationWg.Wait()
		close(invoiceIdCh)
	}()

	var invoiceIds []string
	for invoiceId := range invoiceIdCh {
		invoiceIds = append(invoiceIds, invoiceId)
	}

	var invoiceApproveAndPayWg sync.WaitGroup
	invoiceApproveAndPayWg.Add(len(invoiceIds))
	for _, invoiceId := range invoiceIds {
		go a.approveAndPayInvoice(headers, invoiceId, &invoiceApproveAndPayWg)
	}
	invoiceApproveAndPayWg.Wait()

	elapsed := time.Since(start) / 1000 / 1000 / 1000
	a.logger.Info(fmt.Sprintf("Stopped, elapsed %d seconds", elapsed))
}

func (a application) getAccessToken() string {
	headers := a.client.CreateHeaders()
	bytes, err := json.Marshal(map[string]any{"username": Username, "password": Password})
	if err != nil {
		a.logger.Error(err.Error())
		panic(err)
	}
	resp := a.client.DoPostReturnMapStringAny(a.client.CreateEndpoint("authn/login"), bytes, headers)

	return resp["okapiToken"].(string)
}

func (a application) createBudgetAllocations(headers map[string]string) {
	budgetAllocationPayload := map[string]any{
		"transactionsToCreate": []map[string]any{
			{
				"id":              uuid.New().String(),
				"toFundId":        FundId,
				"amount":          AllocationAmount,
				"transactionType": TransactionType,
				"fiscalYearId":    FiscalYearId,
				"currency":        Currency,
				"source":          Source,
			},
			{
				"id":              uuid.New().String(),
				"toFundId":        FundId2,
				"amount":          AllocationAmount,
				"transactionType": TransactionType,
				"fiscalYearId":    FiscalYearId,
				"currency":        Currency,
				"source":          Source,
			},
		},
	}
	bytes, err := json.Marshal(budgetAllocationPayload)
	if err != nil {
		a.logger.Error(err.Error())
		panic(err)
	}

	a.client.DoPostReturnMapStringAny(a.client.CreateEndpoint("finance/transactions/batch-all-or-nothing"), bytes, headers)
	a.logger.Info("Budget allocations created")
}

func (a application) recalculateBudgets(headers map[string]string) {
	for _, budgetId := range []string{BudgetId, BudgetId2} {
		a.client.DoPostReturnMapStringAny(a.client.CreateEndpoint(fmt.Sprintf("finance/budgets/%s/recalculate", budgetId)), []byte{}, headers)
	}
	a.logger.Info("Budgets recalculated")
}

func (a application) createInvoiceAndInvoiceLines(headers map[string]string, invoiceIdCh chan string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	invoiceId := a.createInvoice(headers)
	a.logger.Info(fmt.Sprintf("Created invoice %s", invoiceId))

	invoiceLineId := a.createInvoiceLine(headers, invoiceId)
	a.logger.Info(fmt.Sprintf("Created invoice line %s", invoiceLineId))

	invoiceIdCh <- invoiceId
}

func (a application) createInvoice(headers map[string]string) string {
	invoicePayload := map[string]any{
		"id":                     uuid.New().String(),
		"chkSubscriptionOverlap": true,
		"currency":               Currency,
		"source":                 Source,
		"adjustments": []map[string]any{{
			"type":            AdjustmentType,
			"description":     "1",
			"value":           AdjustmentValue,
			"relationToTotal": RelationToTotal,
			"prorate":         Prorate,
			"fundDistributions": []map[string]any{{
				"distributionType": DistributionType,
				"value":            DistributionValue,
				"fundId":           FundId,
				"code":             FundCode1,
				"encumbrance":      nil,
				"expenseClassId":   ExpenseClassId,
			}},
		}},
		"batchGroupId":       BatchGroupId,
		"status":             InvoiceStatus,
		"exportToAccounting": true,
		"vendorId":           VendorId,
		"invoiceDate":        time.Now().Format("2006-01-02"),
		"vendorInvoiceNo":    fmt.Sprintf("VENDOR-%d-%d", time.Now().Unix(), rand.Intn(10000)),
		"accountingCode":     AccountingCode,
		"paymentMethod":      PaymentMethod,
		"accountNo":          nil,
	}
	bytes, err := json.Marshal(invoicePayload)
	if err != nil {
		a.logger.Error(err.Error())
		panic(err)
	}
	resp := a.client.DoPostReturnMapStringAny(a.client.CreateEndpoint("invoice/invoices"), bytes, headers)

	return resp["id"].(string)
}

func (a application) createInvoiceLine(headers map[string]string, invoiceId string) string {
	invoiceLinePayload := map[string]any{
		"id":                uuid.New().String(),
		"invoiceId":         invoiceId,
		"invoiceLineStatus": InvoiceStatus,
		"fundDistributions": []map[string]any{{
			"distributionType": DistributionType,
			"value":            DistributionValue,
			"fundId":           FundId2,
			"code":             FundCode2,
			"encumbrance":      nil,
			"expenseClassId":   nil,
		}},
		"releaseEncumbrance": false,
		"description":        InvoiceLineDescription,
		"subTotal":           SubTotal,
		"quantity":           Quantity,
	}
	bytes, err := json.Marshal(invoiceLinePayload)
	if err != nil {
		a.logger.Error(err.Error())
		panic(err)
	}
	resp := a.client.DoPostReturnMapStringAny(a.client.CreateEndpoint("invoice/invoice-lines"), bytes, headers)

	return resp["id"].(string)
}

func (a application) approveAndPayInvoice(headers map[string]string, invoiceId string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	invoiceBeforeApproval := a.getInvoiceById(headers, invoiceId)
	a.logger.Info(fmt.Sprintf("Received invoice %s before approval", invoiceId))

	a.logger.Info(fmt.Sprintf("Approving invoice %s", invoiceId))
	a.approveInvoice(headers, invoiceBeforeApproval)
	a.logger.Info(fmt.Sprintf("Approved invoice %s", invoiceId))

	invoiceBeforePayment := a.getInvoiceById(headers, invoiceId)
	a.logger.Info(fmt.Sprintf("Received invoice %s before payment", invoiceId))

	a.logger.Info(fmt.Sprintf("Paying invoice %s", invoiceId))
	a.payInvoice(headers, invoiceBeforePayment)
	a.logger.Info(fmt.Sprintf("Paid invoice %s", invoiceId))
}

func (a application) getInvoiceById(headers map[string]string, invoiceId string) map[string]any {
	return a.client.DoGetReturnMapStringAny(a.client.CreateEndpoint(fmt.Sprintf("invoice/invoices/%s", invoiceId)), headers)
}

func (a application) approveInvoice(headers map[string]string, invoice map[string]any) {
	invoiceCopy := copyInvoiceWithStatus(invoice, "Approved")

	bytes, err := json.Marshal(invoiceCopy)
	if err != nil {
		a.logger.Error(err.Error())
		panic(err)
	}

	invoiceId := invoiceCopy["id"].(string)
	a.client.DoPutReturnNoContent(a.client.CreateEndpoint(fmt.Sprintf("invoice/invoices/%s", invoiceId)), bytes, headers)
}

func (a application) payInvoice(headers map[string]string, invoice map[string]any) {
	invoiceCopy := copyInvoiceWithStatus(invoice, "Paid")

	bytes, err := json.Marshal(invoiceCopy)
	if err != nil {
		a.logger.Error(err.Error())
		panic(err)
	}

	invoiceId := invoiceCopy["id"].(string)
	a.client.DoPutReturnNoContent(a.client.CreateEndpoint(fmt.Sprintf("invoice/invoices/%s", invoiceId)), bytes, headers)
}

func copyInvoiceWithStatus(invoice map[string]any, status string) map[string]any {
	invoiceCopy := make(map[string]any)
	maps.Copy(invoiceCopy, invoice)
	invoiceCopy["status"] = status

	return invoiceCopy
}
