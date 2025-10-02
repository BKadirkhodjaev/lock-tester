package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/BKadirkhodjaev/lock-tester/util"
	"github.com/google/uuid"
)

const (
	CommandName string = "Main"

	FundId         string = "7fbd5d84-62d1-44c6-9c45-6cb173998bbd"
	FiscalYearId   string = "43aba0df-70e4-435e-8463-d8f730d19e0b"
	BudgetId       string = "076e9607-2181-42a4-94f7-ed379af0f810"
	BatchGroupId   string = "cd592659-77aa-4eb3-ac34-c9a4657bb20f"
	VendorId       string = "e0fb5df2-cdf1-11e8-a8d5-f2801f1b9fd1"
	ExpenseClassId string = "1bcc3247-99bf-4dca-9b0f-7bc51a2998c2"
)

var (
	threadCount int
	enableDebug bool
)

func main() {
	slog.Info(CommandName, util.GetFuncName(), "Started")

	parseArgs()

	token := getAccessToken()
	headers := util.CreateHeadersWithToken(token)

	recalculateBudget(headers)
	createBudgetAllocation(headers)

	var invoiceIds []string
	for range threadCount {
		invoiceId := createInvoiceAndInvoiceLines(token)
		invoiceIds = append(invoiceIds, invoiceId)
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(len(invoiceIds))
	for _, invoiceId := range invoiceIds {
		go approveAndPayInvoice(headers, invoiceId, &waitGroup)
	}
	waitGroup.Wait()
	slog.Info(CommandName, util.GetFuncName(), "Stopped")
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

func createBudgetAllocation(headers map[string]string) {
	budgetAllocationPayload := map[string]any{
		"transactionsToCreate": []map[string]any{{
			"toFundId":        FundId,
			"amount":          "10000",
			"id":              uuid.New().String(),
			"transactionType": "Allocation",
			"fiscalYearId":    FiscalYearId,
			"currency":        "USD",
			"source":          "User",
		}},
	}
	bytes, err := json.Marshal(budgetAllocationPayload)
	if err != nil {
		slog.Error(CommandName, util.GetFuncName(), "json.Marshal error")
		panic(err)
	}

	util.DoPostReturnMapStringInteface(CommandName, util.CreateEndpoint(CommandName, "finance/transactions/batch-all-or-nothing"), enableDebug, bytes, headers)
	slog.Info(CommandName, util.GetFuncName(), "Budget allocation created")
}

func recalculateBudget(headers map[string]string) {
	util.DoPostReturnMapStringInteface(CommandName, util.CreateEndpoint(CommandName, fmt.Sprintf("finance/budgets/%s/recalculate", BudgetId)), enableDebug, []byte{}, headers)
	slog.Info(CommandName, util.GetFuncName(), "Budget recalculated")
}

func createInvoiceAndInvoiceLines(token string) string {
	slog.Info(CommandName, util.GetFuncName(), "Creating invoice and invoice lines")
	headers := util.CreateHeadersWithToken(token)

	invoiceId := createInvoice(headers)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Created invoice with ID: %s", invoiceId))

	invoiceLineId := createInvoiceLine(headers, invoiceId)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Created invoice line with ID: %s", invoiceLineId))

	return invoiceId
}

func createInvoice(headers map[string]string) string {
	invoicePayload := map[string]any{
		"id":                     uuid.New().String(),
		"chkSubscriptionOverlap": true,
		"currency":               "USD",
		"source":                 "User",
		"adjustments":            []any{},
		"batchGroupId":           BatchGroupId,
		"status":                 "Open",
		"exportToAccounting":     true,
		"vendorId":               VendorId,
		"invoiceDate":            time.Now().Format("2006-01-02"),
		"vendorInvoiceNo":        fmt.Sprintf("VENDOR-%d-%d", time.Now().Unix(), rand.Intn(10000)),
		"accountingCode":         "G64758-74834",
		"paymentMethod":          "Credit Card",
		"accountNo":              nil,
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
		"invoiceLineStatus": "Open",
		"fundDistributions": []map[string]any{{
			"distributionType": "percentage",
			"value":            100,
			"fundId":           FundId,
			"code":             "AFRICAHIST",
			"encumbrance":      nil,
			"expenseClassId":   ExpenseClassId,
		}},
		"releaseEncumbrance": false,
		"description":        "Test invoice line",
		"subTotal":           10.0,
		"quantity":           "1",
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
	slog.Info(CommandName, util.GetFuncName(), "Starting approve and pay process")
	defer waitGroup.Done()

	invoiceBeforeApproval := getInvoiceById(headers, invoiceId)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Approving invoice %s", invoiceId))
	approveInvoice(headers, invoiceBeforeApproval)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Approved invoice %s", invoiceId))

	invoiceBeforePayment := getInvoiceById(headers, invoiceId)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Paying invoice %s", invoiceId))
	payInvoice(headers, invoiceBeforePayment)
	slog.Info(CommandName, util.GetFuncName(), fmt.Sprintf("Paid invoice %s", invoiceId))
}

func getInvoiceById(headers map[string]string, invoiceId string) map[string]any {
	return util.DoGetReturnMapStringInterface(CommandName, util.CreateEndpoint(CommandName, fmt.Sprintf("invoice/invoices/%s", invoiceId)), enableDebug, headers)
}

func approveInvoice(headers map[string]string, invoice map[string]any) {
	invoice["status"] = "Approved"

	bytes, err := json.Marshal(invoice)
	if err != nil {
		slog.Error(CommandName, util.GetFuncName(), "json.Marshal error")
		panic(err)
	}

	invoiceId := invoice["id"].(string)
	util.DoPutReturnNoContent(CommandName, util.CreateEndpoint(CommandName, fmt.Sprintf("invoice/invoices/%s", invoiceId)), enableDebug, bytes, headers)
}

func payInvoice(headers map[string]string, invoice map[string]any) {
	invoice["status"] = "Paid"

	bytes, err := json.Marshal(invoice)
	if err != nil {
		slog.Error(CommandName, util.GetFuncName(), "json.Marshal error")
		panic(err)
	}

	invoiceId := invoice["id"].(string)
	util.DoPutReturnNoContent(CommandName, util.CreateEndpoint(CommandName, fmt.Sprintf("invoice/invoices/%s", invoiceId)), enableDebug, bytes, headers)
}
