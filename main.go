package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	// "sync"
	"sync/atomic"

	"SubidaArchivos/views"

	"github.com/joho/godotenv"
	"github.com/martinlindhe/notify"
)

var imgCounter int64 // el acceso a esta variable debe ser concurrente

// var imgCounter atomic.Int64 // Tipo de datos especial para generar accesos concurrentes a esta variable con metodos especiales (lo maneja todo el runtime de Go por defecto)

var validFormats = map[string]bool{
	"image/png":  true,
	"image/jpg":  true,
	"image/jpeg": true,
	"image/heic": true,
	"image/svg":  true,
	"image/webp": true,
}

var secretKey string

var ctx context.Context

// var mutex sync.Mutex

func main() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	imgCounter = 0
	godotenv.Load()
	secretKey = os.Getenv("SECRET_KEY")

	ctx = context.Background()

	imageServer := http.FileServer(http.Dir(os.Getenv("DIR_ARCHIVOS")))
	http.Handle("/imageGetter/", http.StripPrefix("/imageGetter/", imageServer))

	http.HandleFunc("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/images", imagesHandler)
	http.HandleFunc("/download", downloadHandler)
	http.HandleFunc("/delete", deleteHandler)
	http.ListenAndServe(":8080", nil)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20)         // establece que escriba en RAM el contenido de la respuesta si es menor a 20 MB (10 Bytes desplazados 20 bits hacia la izquierda)
	files := r.MultipartForm.File["files"] // obtenemos todos los archivos subidos
	for _, file := range files {

		atomic.AddInt64(&imgCounter, 1) // operacion CONCURRENTE especial de Go para generar operaciones atomicas de incremento de una variable

		// mutex.Lock()
		// aux := imgCounter + 1
		// imgCounter := aux
		// mutex.Unlock()

		content, err := file.Open() // abrimos el archivo
		if err != nil {
			w.Header().Set("HX-Trigger", "failed_open")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		defer content.Close() // cerramos el archivo luego de usarlo

		// leemos el contenido del archivo antes para verificar que se trate efectivamente de un tipo permitido
		buffer := make([]byte, 512) // necesitamos generar un pequeño buffer en memoria RAM de 512 BYTES para leer los primeros bytes del archivo
		_, err = content.Read(buffer)
		if err != nil {
			w.Header().Set("HX-Trigger", "failed_read")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		content.Seek(0, 0) // luego de leer los primeros bytes, volvemos el cabezal de lectura al principio para el momento que se debe copiar

		fileType := http.DetectContentType(buffer) // detectamos el tipo del archivo segun su contenido
		if !validFormats[fileType] {
			http.Error(w, fmt.Sprintf("Contenido no permitido: %s", file.Filename), http.StatusInternalServerError)
			return
		}

		extension := filepath.Ext(file.Filename)                                                                        // sintaxis necesaria para quedarnos con la extension del archivo
		nombre := file.Filename[:len(file.Filename)-len(extension)]                                                     // acortamos el nombre del archivo con la sintaxis string[inicio:fin]
		os.MkdirAll(os.Getenv("DIR_ARCHIVOS"), os.ModePerm)                                                             // creamos la carpeta SOLO SI NO EXISTE con los permisos de lectura y escritura
		created, err := os.Create(fmt.Sprintf("%s/%s(%d)%s", os.Getenv("DIR_ARCHIVOS"), nombre, imgCounter, extension)) // creamos el archivo en nuestra carpeta del servidor
		if err != nil {
			w.Header().Set("HX-Trigger", "failed_create")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		_, err = io.Copy(created, content) // copiamos el contenido del archivo subido por el usuario hacia el archivo creado en nuestro servidor
		if err != nil {
			w.Header().Set("HX-Trigger", "failed_copy")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
	go func() {
		notify.Notify("File Uploader", "New Files", fmt.Sprintf("A total of %d files have been received in the Server", len(files)), "~/Downloads/notification_badge.png")
	}()

	w.WriteHeader(200) // si llegamos aca es porque todos los archivos se subieron con exito
	w.Write([]byte(`
		<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 -960 960 960" ><path d="m424-296 282-282-56-56-226 226-114-114-56 56 170 170Zm56 216q-83 0-156-31.5T197-197q-54-54-85.5-127T80-480q0-83 31.5-156T197-763q54-54 127-85.5T480-880q83 0 156 31.5T763-763q54 54 85.5 127T880-480q0 83-31.5 156T763-197q-54 54-127 85.5T480-80Zm0-80q134 0 227-93t93-227q0-134-93-227t-227-93q-134 0-227 93t-93 227q0 134 93 227t227 93Zm0-320Z"/></svg>
		<span>Fotos subidas con éxito</span>
	`))
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	sk := r.FormValue("secret-key")
	if sk != secretKey {
		w.Header().Set("HX-Trigger", "wrong-key")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	_, err := os.ReadDir(os.Getenv("DIR_ARCHIVOS"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			w.Header().Set("HX-Trigger", "error")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
	w.Header().Set("HX-Redirect", "/images")
	w.WriteHeader(http.StatusOK)
}

func imagesHandler(w http.ResponseWriter, r *http.Request) {
	archivos, err := os.ReadDir(os.Getenv("DIR_ARCHIVOS"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			http.Error(w, "Error al obtener imagenes", http.StatusNoContent)
			return
		}
	}
	views.ListImages(archivos).Render(ctx, w)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment;filename=images.zip")

	writer := zip.NewWriter(w)
	defer writer.Close()

	directorio := os.Getenv("DIR_ARCHIVOS")
	archivos, err := os.ReadDir(directorio)
	if err != nil {
		http.Error(w, "Error al leer carpeta de imagenes", http.StatusBadRequest)
		return
	}
	for _, archivo := range archivos {
		ruta := filepath.Join(directorio, archivo.Name())
		contenido, err := os.Open(ruta)
		if err != nil {
			http.Error(w, "Error al leer imagen", http.StatusBadRequest)
			return
		}
		defer contenido.Close()

		creado, err := writer.Create(archivo.Name())
		if err != nil {
			http.Error(w, "Error al crear imagen dentro de zip", http.StatusBadRequest)
			return
		}
		io.Copy(creado, contenido)
	}

	os.Remove("./images.zip")
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	err := os.RemoveAll(os.Getenv("DIR_ARCHIVOS"))
	if err != nil {
		http.Error(w, "Error al borrar imagenes del servidor", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/images", http.StatusSeeOther)
}
