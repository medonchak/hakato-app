package main

import (
		"archive/zip"
		"archive/unzip"
		"fmt"
		"io"
		"os"
		"path/filepath"
)

func main(){
if len(os.Args) < 3 {
fmt.Println ("назву файлу з ляхом та бажану назву архіву")
return
}
folder:=os.Args[1]
arhiv:=os.Args[2]

zipFile, err :=os.Create(output)
if err != nil{
	panic(err)
}
defer zipFile.Close()

filepath.Walk(folder, func(path string, info os.FileInfo, err error) error){

}
}
