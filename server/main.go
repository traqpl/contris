package main

import (
	"embed"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
)

//go:embed web
var webFS embed.FS

func main() {
	// Ensure .wasm gets correct MIME type (some systems lack it)
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".ogg", "audio/ogg")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8070"
	}

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))

	addr := ":" + port
	log.Printf("CARGO SHIFT → http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
