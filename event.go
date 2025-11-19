package traefik_hawkeye

type Event struct {
	TS          string            `json:"ts"`
	IP          string            `json:"ip"`
	Method      string            `json:"method"`
	Scheme      string            `json:"scheme"`
	Host        string            `json:"host"`
	Path        string            `json:"path"`
	Status      int               `json:"status"`
	DurMs       int64             `json:"dur_ms"`
	Ref         string            `json:"ref"`
	UA          string            `json:"ua"`
	ContentType string            `json:"content_type"`
	RequestHdr  map[string]string `json:"request_hdr"`
	ResponseHdr map[string]string `json:"response_hdr"`
}
