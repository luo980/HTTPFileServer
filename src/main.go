package main

import (
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"fmt"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

const (
	dir 			= 		"/home/luo980/mygits/file-update-ota/serve_path"
	FILE 			= 		"File"
	FOLDER 			= 		"Folder"
)

type File struct {
	ID				int 	`json:"id"`
	Title			string	`json:"title"`
	Type			string	`json:"type"`
	SHA256			string	`json:"sha256"`
}

func createFileJSON(id int, title string, fileType string, hash256 string) *File {
	file := new(File)
	file.ID = id
	file.Title = title
	file.Type = fileType
	file.SHA256 = hash256
	return file
}

func calculateFileSHA256(filePath string) (string, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return "", err
    }
    defer file.Close()

    hash256 := sha256.New()
    if _, err := io.Copy(hash256, file); err != nil {
        return "", err
    }

    hash := hash256.Sum(nil)
    return hex.EncodeToString(hash), nil
}


func (file File) addToJSON(files []File) []File {
	files = append(files, file)
	return files
}

func defineFileOrFolder(filename os.FileInfo) string {
	if filename.IsDir() {
		return FOLDER
	} else {
		return FILE
	}
}

func copyFile(c echo.Context, src multipart.File, path string) error {
	dst, err := os.Create(path)
	if err != nil {
		c.Response().WriteHeader(http.StatusBadRequest)
		return err
	}
	defer dst.Close()
	if _, err = io.Copy(dst, src); err != nil {
		c.Response().WriteHeader(http.StatusBadRequest)
		return err
	}
	return nil
}

func getFiles(c echo.Context) {
	path := dir + c.Request().RequestURI
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Print(err)
		c.Response().WriteHeader(http.StatusNotFound)
	}

	var foldersFiles []File
	// var file File

    for index, f := range files {
        file := File{
            ID:    index,
            Title: f.Name(),
            Type:  defineFileOrFolder(f),
        }

        if !f.IsDir() { // 只计算文件的 SHA256，跳过文件夹
            sha, err := calculateFileSHA256(filepath.Join(path, f.Name()))
            if err == nil {
                file.SHA256 = sha
            } else {
                log.Printf("Error calculating SHA256 for file %s: %v", f.Name(), err)
                // 可以选择如何处理这个错误，比如跳过或返回错误信息
            }
        }

        foldersFiles = append(foldersFiles, file)
    }

	c.JSON(http.StatusOK, foldersFiles)
}

func getFile(c echo.Context) {
	c.File(dir + c.Request().RequestURI)
	c.Response().WriteHeader(http.StatusOK)
}

func handleGETMethod(c echo.Context) error {
	path := dir + c.Request().RequestURI
	fi, err := os.Stat(path)
	if err != nil {
		c.Response().WriteHeader(http.StatusNotFound)
		return nil
	}

	if fi.IsDir() {
		getFiles(c)
	} else {
		getFile(c)
	}
	return nil
}

func handlePUTMethod(c echo.Context) error {
    fmt.Println("handlePUTMethod")
    fmt.Println(dir + c.Request().RequestURI)
    _, err := os.Stat(dir + c.Request().RequestURI)
    if err != nil {
        c.Response().WriteHeader(http.StatusNotFound)
        fmt.Println(err)
        return nil
    }

    fmt.Println("handleFileForm")

    file, err := c.FormFile(FILE)
    if err != nil {
        fmt.Println(err)
        c.Response().WriteHeader(http.StatusNotFound)
        return err
    }
    src, err := file.Open()
    if err != nil {
        return err
    }
    defer src.Close()

    // 计算 SHA256 哈希
    hash256 := sha256.New()
    if _, err := io.Copy(hash256, src); err != nil {
        return err
    }

    hash := hash256.Sum(nil)
    sha256String := hex.EncodeToString(hash)

    // 重置 src 的读取指针到文件开头
    if _, err := src.Seek(0, 0); err != nil {
        return err
    }

    path := dir + c.Request().RequestURI + file.Filename

    // 复制文件
    err = copyFile(c, src, path)
    if err != nil {
        return err
    }

    json := createFileJSON(0, file.Filename, FILE, sha256String)
    return c.JSON(http.StatusOK, json)
}


func handleDELETEMethod(c echo.Context) error {
	path := dir + c.Request().RequestURI

	_, err := os.Stat(path)
	if err != nil {
		c.Response().WriteHeader(http.StatusNotFound)
		return nil
	}

	err = os.RemoveAll(path)
	if err != nil {
		c.Response().WriteHeader(http.StatusBadRequest)
		return nil
	}

	c.Response().WriteHeader(http.StatusOK)
	return nil
}

func handleHEADMethod(c echo.Context) error {
	path := dir + c.Request().RequestURI

	fi, err := os.Stat(path)
	if err != nil {
		c.Response().WriteHeader(http.StatusNotFound)
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		c.Response().WriteHeader(http.StatusBadRequest)
		return nil
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil {
		c.Response().WriteHeader(http.StatusBadRequest)
		return nil
	}

	fileSize := strconv.FormatInt(fi.Size(), 10)

	c.Response().Header().Set("Connection", "Closed")
	c.Response().Header().Add(echo.HeaderContentLength, fileSize)
	c.Response().Header().Add(echo.HeaderContentType, http.DetectContentType(buffer[:n]))
	c.Response().Header().Add(echo.HeaderContentDisposition, "attachment; filename=\"" + fi.Name() + "\"")
	c.Response().Header().Add("File Server", "shine")
	c.Response().WriteHeader(http.StatusOK)
	return nil
}

func handlePOSTMethod(c echo.Context) error {
	currPath := dir + c.Request().RequestURI;
	nextPath := dir + c.Request().Header.Get("X-Copy-From")

	file, err := os.Open(currPath)
	if err != nil {
		c.Response().WriteHeader(http.StatusNotFound)
		return err
	}

	defer file.Close()

	err = copyFile(c, file, nextPath)
	if err != nil {
		c.Response().WriteHeader(http.StatusBadRequest)
		return nil
	}
	c.Response().WriteHeader(http.StatusOK)
	return nil
}

func main() {
	e := echo.New()
	e.GET("*", handleGETMethod)
	e.PUT("*", handlePUTMethod)
	e.DELETE("*", handleDELETEMethod)
	e.HEAD("*", handleHEADMethod)
	e.POST("*", handlePOSTMethod)
	e.Logger.Fatal(e.Start(":1323"))
}