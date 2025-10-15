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
	flag.BoolVar(&enableDebug, "debug", false, "Enable debug output of HTTP payload, request and response")
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

	app := &application{logger: logger, client: &client.RequestClient{
		Logger:       logger,
		URI:          URI,
		Tenant:       Tenant,
		RetryMax:     30,
		RetryWaitMax: 500 * time.Second,
	}}

	app.logger.Info("Started", "threads", threadCount, "debug", enableDebug)

	token, err := app.getAccessToken()
	if err != nil {
		app.logger.Error("failed to receive an access token", "err", err.Error())
		os.Exit(1)
	}

	headers := app.client.CreateHeadersWithToken(token)

	err = app.recalculateBudgets(headers)
	if err != nil {
		app.logger.Error("failed to recalculate budgets", "err", err.Error())
		os.Exit(1)
	}

	err = app.createBudgetAllocations(headers)
	if err != nil {
		app.logger.Error("faied to create budget allocations", "err", err.Error())
		os.Exit(1)
	}

	invoiceIdCh := make(chan string, threadCount)
	var invoiceCreationWg sync.WaitGroup
	invoiceCreationWg.Add(threadCount)
	for range threadCount {
		go app.createInvoiceAndInvoiceLines(headers, invoiceIdCh, &invoiceCreationWg)
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
		go app.approveAndPayInvoice(headers, invoiceId, &invoiceApproveAndPayWg)
	}
	invoiceApproveAndPayWg.Wait()

	elapsed := time.Since(start) / 1000 / 1000 / 1000
	app.logger.Info(fmt.Sprintf("Stopped, elapsed %d seconds", elapsed))
}

func (app *application) getAccessToken() (string, error) {
	headers := app.client.CreateHeaders()
	bytes, err := json.Marshal(map[string]any{"username": Username, "password": Password})
	if err != nil {
		return "", err
	}

	endpoint, err := app.client.CreateEndpoint("authn/login")
	if err != nil {
		return "", err
	}

	resp, err := app.client.DoPostReturnMapStringAny(endpoint, bytes, headers)
	if err != nil {
		return "", err
	}

	return resp["okapiToken"].(string), nil
}

func (app *application) createBudgetAllocations(headers map[string]string) error {
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
		return err
	}

	endpoint, err := app.client.CreateEndpoint("finance/transactions/batch-all-or-nothing")
	if err != nil {
		return err
	}

	_, err = app.client.DoPostReturnMapStringAny(endpoint, bytes, headers)
	if err != nil {
		return err
	}
	app.logger.Info("Budget allocations created")

	return nil
}

func (app *application) recalculateBudgets(headers map[string]string) error {
	for _, budgetId := range []string{BudgetId, BudgetId2} {
		endpoint, err := app.client.CreateEndpoint(fmt.Sprintf("finance/budgets/%s/recalculate", budgetId))
		if err != nil {
			return err
		}

		_, err = app.client.DoPostReturnMapStringAny(endpoint, []byte{}, headers)
		if err != nil {
			return err
		}
	}
	app.logger.Info("Budgets recalculated")

	return nil
}

func (app *application) createInvoiceAndInvoiceLines(headers map[string]string, invoiceIdCh chan string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	invoiceId, err := app.createInvoice(headers)
	if err != nil {
		app.logger.Error("failed to create an invoice", "err", err.Error())
		return
	}
	app.logger.Info(fmt.Sprintf("Created invoice %s", invoiceId))

	invoiceLineId, err := app.createInvoiceLine(headers, invoiceId)
	if err != nil {
		app.logger.Error("failed to create an invoice line", "err", err.Error())
		return
	}
	app.logger.Info(fmt.Sprintf("Created invoice line %s", invoiceLineId))

	invoiceIdCh <- invoiceId
}

func (app *application) createInvoice(headers map[string]string) (string, error) {
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
		return "", err
	}

	endpoint, err := app.client.CreateEndpoint("invoice/invoices")
	if err != nil {
		return "", err
	}

	resp, err := app.client.DoPostReturnMapStringAny(endpoint, bytes, headers)
	if err != nil {
		return "", err
	}

	return resp["id"].(string), nil
}

func (app *application) createInvoiceLine(headers map[string]string, invoiceId string) (string, error) {
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
		return "", err
	}

	endpoint, err := app.client.CreateEndpoint("invoice/invoice-lines")
	if err != nil {
		return "", err
	}

	resp, err := app.client.DoPostReturnMapStringAny(endpoint, bytes, headers)
	if err != nil {
		return "", err
	}

	return resp["id"].(string), nil
}

func (app *application) approveAndPayInvoice(headers map[string]string, invoiceId string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	invoiceBeforeApproval, err := app.getInvoiceById(headers, invoiceId)
	if err != nil {
		app.logger.Error("failed to receive an invoice by id before approval", "err", err.Error())
		return
	}
	app.logger.Info(fmt.Sprintf("Received invoice %s before approval", invoiceId))

	app.logger.Info(fmt.Sprintf("Approving invoice %s", invoiceId))
	err = app.approveOrPayInvoice(headers, invoiceBeforeApproval, "Approved")
	if err != nil {
		app.logger.Error("failed to approve an invoice", "err", err.Error())
		return
	}
	app.logger.Info(fmt.Sprintf("Approved invoice %s", invoiceId))

	invoiceBeforePayment, err := app.getInvoiceById(headers, invoiceId)
	if err != nil {
		app.logger.Error("failed to receive an invoice by id before payment", "err", err.Error())
		return
	}
	app.logger.Info(fmt.Sprintf("Received invoice %s before payment", invoiceId))

	app.logger.Info(fmt.Sprintf("Paying invoice %s", invoiceId))
	err = app.approveOrPayInvoice(headers, invoiceBeforePayment, "Paid")
	if err != nil {
		app.logger.Error("failed to pay an invoice", "err", err.Error())
		return
	}
	app.logger.Info(fmt.Sprintf("Paid invoice %s", invoiceId))
}

func (app *application) getInvoiceById(headers map[string]string, invoiceId string) (map[string]any, error) {
	endpoint, err := app.client.CreateEndpoint(fmt.Sprintf("invoice/invoices/%s", invoiceId))
	if err != nil {
		return nil, err
	}

	return app.client.DoGetReturnMapStringAny(endpoint, headers)
}

func (app *application) approveOrPayInvoice(headers map[string]string, invoice map[string]any, status string) error {
	invoiceCopy := copyInvoiceWithStatus(invoice, status)

	bytes, err := json.Marshal(invoiceCopy)
	if err != nil {
		return err
	}

	invoiceId := invoiceCopy["id"].(string)

	endpoint, err := app.client.CreateEndpoint(fmt.Sprintf("invoice/invoices/%s", invoiceId))
	if err != nil {
		return err
	}

	err = app.client.DoPutReturnNoContent(endpoint, bytes, headers)
	if err != nil {
		return err
	}

	return nil
}

func copyInvoiceWithStatus(invoice map[string]any, status string) map[string]any {
	invoiceCopy := make(map[string]any)
	maps.Copy(invoiceCopy, invoice)
	invoiceCopy["status"] = status

	return invoiceCopy
}
