package ingestion

type AlertmanagerPayload struct {
	Alerts []Alert `json:"alerts"`
}

type Alert struct {
	Status      string            `json:"status"`
	Fingerprint string            `json:"fingerprint"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type Event struct {
	Fingerprint    string
	CorrelationKey string
	WorkloadKind   string
	Namespace      string
	Name           string
	Reason         string
}
