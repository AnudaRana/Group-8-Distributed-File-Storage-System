package replication

type FileData struct {
	Name      string `json:"name"`
	Content   string `json:"content"`
	Version   int    `json:"version"`
	Timestamp int64  `json:"timestamp"`
}

type NodeSnapshot struct {
	NodeID string              `json:"nodeID"`
	Files  map[string]FileData `json:"files"`
}
