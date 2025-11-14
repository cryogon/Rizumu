// Package utils: utitility method
package utils

import (
	"fmt"
	"net/http"
)

var HTTPClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		fmt.Printf("Redirect to: %s\n", req.URL.String())
		return nil
	},
}
