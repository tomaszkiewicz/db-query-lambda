package payload

type QueryRequest struct {
	Query    string `json:"query"`
	Database string `json:"database"`
}

type QueryResponse struct {
	Rows []map[string]string `json:"rows"`
}
