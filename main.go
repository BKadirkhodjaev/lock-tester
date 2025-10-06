package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"math/rand"
	"sync"
	"time"

	"github.com/BKadirkhodjaev/lock-tester/util"
	"github.com/google/uuid"
)

const (
	CommandName string = "Main"

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
	AllocationAmount  string  = "10000"
	AdjustmentValue   int     = 10
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

func main() {
	start := time.Now()
	slog.Info(CommandName, util.GetFuncName(), "Started")

	parseArgs()

	token := getAccessToken()
	headers := util.CreateHeadersWithToken(token)

	recalculateBudgets(headers)
	createBudgetAllocations(headers)

	invoiceIdCh := make(chan string, threadCount)
	var invoiceCreationWaitGroup sync.WaitGroup
	invoiceCreationWaitGroup.Add(threadCount)
	for range threadCount {
		go createInvoiceAndInvoiceLines(headers, invoiceIdCh, &invoiceCreationWaitGroup)
	}

	go func() {
		invoiceCreationWaitGroup.Wait()
		close(invoiceIdCh)
	}()

	var invoiceIds []string
	for invoiceId := range invoiceIdCh {
		invoiceIds = append(invoiceIds, invoiceId)
	}

	var invoiceApproveAndPayWaitGroup sync.WaitGroup
	invoiceApproveAndPayWaitGroup.Add(len(invoiceIds))
	for _, invoiceId := range invoiceIds {
		go approveAndPayInvoice(headers, invoiceId, &invoiceApproveAndPayWaitGroup)
	}
	invoiceApproveAndPayWaitGroup.Wait()

	elapsed := time.Since(start) / 1000 / 1000 / 1000
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Stopped, elapsed %d seconds", elapsed))
}

func parseArgs() {
	flag.IntVar(&threadCount, "threads", 50, "Persistent HTTP thread count")
	flag.BoolVar(&enableDebug, "debug", false, "Enable debug output of HTTP request and response")
	flag.Parse()

	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Enable debug: %t", enableDebug))
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Thread count: %d", threadCount))
}

func getAccessToken() string {
	headers := util.CreateHeaders()
	bytes, err := json.Marshal(map[string]any{"username": "diku_admin", "password": "admin"})
	if err != nil {
		slog.Error(CommandName, util.GetFuncName(), "json.Marshal error")
		panic(err)
	}
	resp := util.DoPostReturnMapStringInteface(CommandName, util.CreateEndpoint(CommandName, "authn/login"), enableDebug, bytes, headers)

	return resp["okapiToken"].(string)
}

func createBudgetAllocations(headers map[string]string) {
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
		slog.Error(CommandName, util.GetFuncName(), "json.Marshal error")
		panic(err)
	}

	util.DoPostReturnMapStringInteface(CommandName, util.CreateEndpoint(CommandName, "finance/transactions/batch-all-or-nothing"), enableDebug, bytes, headers)
	slog.Info(CommandName, util.GetFuncName(), "Budget allocations created")
}

func recalculateBudgets(headers map[string]string) {
	for _, budgetId := range []string{BudgetId, BudgetId2} {
		util.DoPostReturnMapStringInteface(CommandName, util.CreateEndpoint(CommandName, fmt.Sprintf("finance/budgets/%s/recalculate", budgetId)), enableDebug, []byte{}, headers)
	}
	slog.Info(CommandName, util.GetFuncName(), "Budgets recalculated")
}

func createInvoiceAndInvoiceLines(headers map[string]string, invoiceIdCh chan string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	invoiceId := createInvoice(headers)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Created invoice %s", invoiceId))

	invoiceLineId := createInvoiceLine(headers, invoiceId)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Created invoice line %s", invoiceLineId))

	invoiceIdCh <- invoiceId
}

func createInvoice(headers map[string]string) string {
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
		slog.Error(CommandName, util.GetFuncName(), "json.Marshal error")
		panic(err)
	}
	resp := util.DoPostReturnMapStringInteface(CommandName, util.CreateEndpoint(CommandName, "invoice/invoices"), enableDebug, bytes, headers)

	return resp["id"].(string)
}

func createInvoiceLine(headers map[string]string, invoiceId string) string {
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
		slog.Error(CommandName, util.GetFuncName(), "json.Marshal error")
		panic(err)
	}
	resp := util.DoPostReturnMapStringInteface(CommandName, util.CreateEndpoint(CommandName, "invoice/invoice-lines"), enableDebug, bytes, headers)

	return resp["id"].(string)
}

func approveAndPayInvoice(headers map[string]string, invoiceId string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	invoiceBeforeApproval := getInvoiceById(headers, invoiceId)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Received invoice %s before approval", invoiceId))

	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Approving invoice %s", invoiceId))
	approveInvoice(headers, invoiceBeforeApproval)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Approved invoice %s", invoiceId))

	invoiceBeforePayment := getInvoiceById(headers, invoiceId)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Received invoice %s before payment", invoiceId))

	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Paying invoice %s", invoiceId))
	payInvoice(headers, invoiceBeforePayment)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Paid invoice %s", invoiceId))
}

func getInvoiceById(headers map[string]string, invoiceId string) map[string]any {
	return util.DoGetReturnMapStringInterface(CommandName, util.CreateEndpoint(CommandName, fmt.Sprintf("invoice/invoices/%s", invoiceId)), enableDebug, headers)
}

func approveInvoice(headers map[string]string, invoice map[string]any) {
	invoiceCopy := copyInvoiceWithStatus(invoice, "Approved")

	bytes, err := json.Marshal(invoiceCopy)
	if err != nil {
		slog.Error(CommandName, util.GetFuncName(), "json.Marshal error")
		panic(err)
	}

	invoiceId := invoiceCopy["id"].(string)
	util.DoPutReturnNoContent(CommandName, util.CreateEndpoint(CommandName, fmt.Sprintf("invoice/invoices/%s", invoiceId)), enableDebug, bytes, headers)
}

func payInvoice(headers map[string]string, invoice map[string]any) {
	invoiceCopy := copyInvoiceWithStatus(invoice, "Paid")

	bytes, err := json.Marshal(invoiceCopy)
	if err != nil {
		slog.Error(CommandName, util.GetFuncName(), "json.Marshal error")
		panic(err)
	}

	invoiceId := invoiceCopy["id"].(string)
	util.DoPutReturnNoContent(CommandName, util.CreateEndpoint(CommandName, fmt.Sprintf("invoice/invoices/%s", invoiceId)), enableDebug, bytes, headers)
}

func copyInvoiceWithStatus(invoice map[string]any, status string) map[string]any {
	invoiceCopy := make(map[string]any)
	maps.Copy(invoiceCopy, invoice)
	invoiceCopy["status"] = status

	return invoiceCopy
}
