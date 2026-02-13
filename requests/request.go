package requests

type Request interface {
	Validate() (bool, string, string)
	Methods() []string
	Path() (string, bool)
	String() string
}

// OpenAPIDoc is optional. If a Request implements it, the values are used in the dynamic OpenAPI spec.
type OpenAPIDoc interface {
	OpenAPISummary() string
	OpenAPIDescription() string
}

// OpenAPIBody is optional. If a Request implements it, the string is used as the request body example in /api/swagger.html.
// It should be valid JSON matching the request structure (e.g. empty fields).
type OpenAPIBody interface {
	OpenAPIBody() string
}

// OpenAPIResponses is optional. If a Request implements it, the map is used in /api/swagger.html for response examples.
// Key is HTTP status code, value is JSON string example. If not implemented, default SuccessResponse is shown.
type OpenAPIResponses interface {
	OpenAPIResponses() map[int]string
}
