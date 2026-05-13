package exclusivenetworks

// Quote represents an Exclusive Networks sales quote.
//
// Multiple versions of the same quoteNumber can exist; only the row with
// IsLatestVersion == true is convertible to a sales order upstream.
type Quote struct {
	ID                     string      `json:"id"`
	QuoteNumber            string      `json:"quoteNumber"`
	Version                int         `json:"version"`
	IsLatestVersion        bool        `json:"isLatestVersion"`
	LastModifiedDateTime   string      `json:"lastModifiedDateTime"`
	Status                 string      `json:"status"`
	CustomerQuoteReference string      `json:"customerQuoteReference"`
	Vendor                 string      `json:"vendor"`
	ExpiryDate             Date        `json:"expiryDate"`
	DealType               string      `json:"dealType"`
	Lines                  []QuoteLine `json:"lines"`
}

// QuoteLine represents a single line on a sales quote.
//
// "Description" lines (per AccessNow §1.2.2) carry free-form text instead
// of a real item — they have ItemName == "Description" and
// VendorPartNumber == "Description". Callers that consume QuoteLine for
// asset/coverage purposes should skip these.
type QuoteLine struct {
	ID                    string  `json:"id"`
	SalesQuoteID          string  `json:"salesQuoteId"`
	LineSequenceNumber    int     `json:"lineSequenceNumber"`
	VendorID              string  `json:"vendorId"`
	Vendor                string  `json:"vendor"`
	ItemName              string  `json:"itemName"`
	Description           string  `json:"description"`
	Quantity              float64 `json:"quantity"`
	ItemType              string  `json:"itemType"`
	VendorPartNumber      string  `json:"vendorPartNumber"`
	SerialNumberSupported string  `json:"serialNumberSupported"`
	ContractStartDate     Date    `json:"contractStartDate"`
	ContractEndDate       Date    `json:"contractEndDate"`
	ManufactureID         string  `json:"manufactureId"`
	ManufactureName       string  `json:"manufactureName"`
	SubscriptionTerm      int     `json:"subscriptionTerm"`
	UnitPrice             float64 `json:"unitPrice"`
	Amount                float64 `json:"amount"`
	Currency              string  `json:"currency"`
}

// Logger is the minimal logging interface accepted by WithLogger.
type Logger interface {
	Printf(format string, args ...any)
}
