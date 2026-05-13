package exclusivenetworks

import "context"

// findQuoteByNumberQuery selects all fields needed for downstream
// coverage-sync use. Adjust this in lockstep with the QuoteLine struct.
const findQuoteByNumberQuery = `query FindQuoteByNumber($number: String!) {
  salesQuotes(find: { quoteNumber: { operator: IS, value: $number } }) {
    id
    quoteNumber
    version
    isLatestVersion
    lastModifiedDateTime
    status
    customerQuoteReference
    vendor
    expiryDate
    dealType
    lines {
      id
      salesQuoteId
      lineSequenceNumber
      vendorId
      vendor
      itemName
      description
      quantity
      itemType
      vendorPartNumber
      serialNumberSupported
      contractStartDate
      contractEndDate
      manufactureId
      manufactureName
      subscriptionTerm
      unitPrice
      amount
      currency
    }
  }
}`

// GetQuoteByNumber resolves a sales quote by its quoteNumber.
//
// The upstream API can return multiple rows when a quote has been
// versioned (only IsLatestVersion == true is convertible to an order
// upstream). This method returns the latest-version row.
//
// Returns ErrQuoteNotFound if no row matches or no matching row has
// IsLatestVersion == true, and ErrAmbiguousQuoteNumber if more than one
// IsLatestVersion == true row exists for the same quoteNumber.
func (c *Client) GetQuoteByNumber(ctx context.Context, quoteNumber string) (*Quote, error) {
	var out struct {
		SalesQuotes []Quote `json:"salesQuotes"`
	}
	err := c.do(ctx, "FindQuoteByNumber", findQuoteByNumberQuery,
		map[string]any{"number": quoteNumber}, &out)
	if err != nil {
		return nil, err
	}
	var latest *Quote
	for i := range out.SalesQuotes {
		if !out.SalesQuotes[i].IsLatestVersion {
			continue
		}
		if latest != nil {
			return nil, ErrAmbiguousQuoteNumber
		}
		latest = &out.SalesQuotes[i]
	}
	if latest == nil {
		return nil, ErrQuoteNotFound
	}
	return latest, nil
}
