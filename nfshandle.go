package main

import (
    "crypto/sha256"
    "path/filepath"
    "sync"
)

var (
    fhMu     sync.Mutex
    fhToPath = make(map[string]string)
)

func normalizePath(p string) string {
    return filepath.Clean(p)
}

func handleForPath(path string) []byte {
    fhMu.Lock()
    defer fhMu.Unlock()
    norm := normalizePath(path)
    sum := sha256.Sum256([]byte(norm))
    fh := sum[:]
    fh32 := make([]byte, 32)
    copy(fh32, fh)
    fhToPath[string(fh32)] = norm
    return fh32
}

func pathForHandle(fh []byte) (string, bool) {
    fhMu.Lock()
    defer fhMu.Unlock()
    p, ok := fhToPath[string(fh)]
    return p, ok
}


