package main

import (
	"fmt"
	//"net/http"
	//"os"
	//"path"
	//"strings"
	//"encoding/json"
	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/s3"
	"time"
)

type Lurker struct {
	dao      *dbl.DAO
	s3       *s3.S3AO
	interval time.Duration
}

func (l *Lurker) Init(interval int) (err error) {
	l.interval = time.Second * time.Duration(interval)
	if err != nil {
		return err
	}
	return nil
}

func (l *Lurker) Run() {
	fmt.Printf("Starting Lurker process (interval: %s)\n", l.interval.String())
	ticker := time.NewTicker(l.interval)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case _ = <-ticker.C:
				t0 := time.Now()
				//l.HideExpiredBins()
				l.DeletePendingFiles()
				l.DeletePendingBins()
				fmt.Printf("Lurker completed run in %.3fs\n", time.Since(t0).Seconds())
			}
		}
	}()
}

//func (l *Lurker) HideExpiredBins() {
//	count, err := l.dao.Bin().HideRecentlyExpiredBins()
//	if err != nil {
//		fmt.Printf("Unable to HideRecentlyExpiredBins(): %s\n", err.Error())
//		return
//	}
//	if count > 0 {
//		fmt.Printf("Hid %d expired bins waiting for deletion.\n", count)
//	}
//}

//func (l *Lurker) HideEmptyBins() {
//	count, err := l.dao.Bin().HideEmptyBins()
//	if err != nil {
//		fmt.Printf("Unable to HideEmptyBins(): %s\n", err.Error())
//		return
//	}
//	if count > 0 {
//		fmt.Printf("Hid %d empty bins waiting for deletion.\n", count)
//	}
//}

func (l *Lurker) DeletePendingFiles() {
	files, err := l.dao.File().GetPendingDelete()
	if err != nil {
		fmt.Printf("Unable to GetPendingDelete(): %s\n", err.Error())
		return
	}
	if len(files) > 0 {
		fmt.Printf("Found %d files pending removal.\n", len(files))
		for _, file := range files {
			if err := l.s3.RemoveObject(file.Bin, file.Filename); err != nil {
				fmt.Printf("Unable to delete file %s from bin %s from S3.\n", file.Filename, file.Bin)
				return
			} else {
				file.InStorage = false
				if err := l.dao.File().Update(&file); err != nil {
					fmt.Printf("Unable to update filename %s (id %d) in bin %s: %s\n", file.Filename, file.Id, file.Bin, err.Error())
					return
				}
			}
		}
	}
}

func (l *Lurker) DeletePendingBins() {
	bins, err := l.dao.Bin().GetPendingDelete()
	if err != nil {
		fmt.Printf("Unable to GetPendingDelete(): %s\n", err.Error())
		return
	}
	if len(bins) > 0 {
		fmt.Printf("Found %d bins pending removal.\n", len(bins))
		for _, bin := range bins {
			files, err := l.dao.File().GetByBin(bin.Id, false)
			if err != nil {
				fmt.Printf("Unable to GetByBin: %s\n", err.Error())
				return
			}
			for _, file := range files {
				if err := l.s3.RemoveObject(file.Bin, file.Filename); err != nil {
					fmt.Printf("Unable to delete file %s from bin %s from S3.\n", file.Filename, file.Bin)
					return
				} else {
					file.InStorage = false
					if err := l.dao.File().Update(&file); err != nil {
						fmt.Printf("Unable to update filename %s (id %d) in bin %s: %s\n", file.Filename, file.Id, file.Bin, err.Error())
						return
					}
				}
			}
			if err := l.dao.Bin().Update(&bin); err != nil {
				fmt.Printf("Unable to update bin %s: %s\n", bin.Id, err.Error())
				return
			}
		}
	}
}
