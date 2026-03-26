package types

type FileEntry struct {
	Path      string
	Data      []byte
	Version   int
	Timestamp float64
	OwnerID   string
	IsDir     bool
}

func NewFileEntry(path string, data []byte, ownerID string) *FileEntry {
	return &FileEntry{
		Path:    path,
		Data:    data,
		Version: 1,
		OwnerID: ownerID,
		IsDir:   false,
	}
}