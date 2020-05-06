package ds

import (
	"time"
)

type File struct {
	Id              int       `json:"-"`
	Bin             string    `json:"-"`
	Filename        string    `json:"filename"`
	Status          int       `json:"-"`
	Mime            string    `json:"content-type"`
	Category        string    `json:"-"`
	Bytes           uint64    `json:"bytes"`
	BytesReadable   string    `json:"bytes_readable"`
	MD5             string    `json:"md5"`
	SHA256          string    `json:"sha256"`
	Downloads       uint64    `json:"-"`
	Updates         uint64    `json:"-"`
	Nonce           []byte    `json:"-"`
	Updated         time.Time `json:"updated"`
	UpdatedRelative string    `json:"updated_relative"`
	Created         time.Time `json:"created"`
	CreatedRelative string    `json:"created_relative"`
	Deleted         time.Time `json:"-"`
	DeletedRelative string    `json:"-"`
	URL             string    `json:"-"`
}

func (f *File) IsReadable() bool {
	// Not readable if the deleted timestamp is more recent than zero
	if f.Deleted.IsZero() == false {
		return false
	}
	// Not readable if flagged as deleted
	if f.Status != 0 {
		return false
	}
	return true
}
