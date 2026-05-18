# exclusivenetworks

Go client library for the [Exclusive Networks](https://www.exclusive-networks.com/) AccessNow GraphQL API.

## Installation

```bash
go get github.com/enthus-golang/exclusivenetworks
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/enthus-golang/exclusivenetworks"
)

func main() {
	client := exclusivenetworks.New(
		"https://YOUR_GRAPHQL_BASE_URL",
		"https://YOUR_OAUTH_TOKEN_URL",
		"YOUR_CLIENT_ID",
		"YOUR_CLIENT_SECRET",
		"YOUR_SCOPE",
	)

	quote, err := client.GetQuoteByNumber(context.Background(), "QPL010006170")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("quote %s version %d\n", quote.QuoteNumber, quote.Version)

	for _, line := range quote.Lines {
		fmt.Printf("  %s %s — %s..%s\n",
			line.VendorPartNumber,
			line.SerialNumberSupported,
			line.ContractStartDate.Format("2006-01-02"),
			line.ContractEndDate.Format("2006-01-02"),
		)
	}
}
```

## Options

```go
client := exclusivenetworks.New(baseURL, tokenURL, clientID, clientSecret, scope,
	exclusivenetworks.WithHTTPClient(myHTTPClient),
	exclusivenetworks.WithRateLimit(60, 3600),               // per minute, per hour (0 = unlimited)
	exclusivenetworks.WithRetry(3, 500*time.Millisecond),    // max attempts, base backoff
)
```

## License

[Apache License 2.0](./LICENSE)
