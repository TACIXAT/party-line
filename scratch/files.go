package main

import (
	"fmt"
	"os"
	"path/filepath"
	"encoding/json"
	"io/ioutil"
	"log"
	"strings"
	"crypto/sha256"
	"io"
)

/*
	peers send signed packs
	client show packs with sig counts
	user selects a pack
	user requests file(s) from pack
	pieces come in
	client constructs chains
		map[hash of block] block
		block { hash of next block }
*/

type PackFile struct {
	Name   string
	Files  []string
}

type Pack struct {
	Name   string
	Files  []string
	Hashes []string
	FirstBlockHashes []string
}

type File struct {
	Path     string
	Sha256   string
	Coverage []uint64
}

var sharedDirAbs string

func sha256File(targetFile *os.File) string {
	h := sha256.New()
	if _, err := io.Copy(h, targetFile); err != nil {
		log.Fatal(err)
	}

	sha256 := fmt.Sprintf("%x", h.Sum(nil))
	return sha256
}

func unpackFile(targetFile *os.File) *PackFile {
	contents, err := ioutil.ReadAll(targetFile)
	if err != nil {
		log.Fatal(err)
	}

	packFile := new(PackFile)
	err = json.Unmarshal(contents, packFile)
	if err != nil {
		log.Fatal(err)
	}

	return packFile
}

func buildPack(path string, targetFile *os.File) {
	pack := new(Pack)

	packFile := unpackFile(targetFile)
	fmt.Println("Name:", packFile.Name)

	if len(packFile.Files) == 0 {
		log.Fatal("no files in pack")
	}

	pack.Name = packFile.Name

	dirPath := filepath.Dir(path)
	for _, shortFilePath := range packFile.Files {
		sharedFilePath := filepath.Join(dirPath, shortFilePath)
		sharedFilePathAbs, err := filepath.Abs(sharedFilePath)
		if err != nil {
			log.Fatal(err)
		}

		if !strings.HasPrefix(sharedFilePathAbs, sharedDirAbs) {
			log.Fatal(
				"bad pack " + packFile.Name + 
				" file outside of file dir " + sharedFilePathAbs)
		}

		relativePath := sharedFilePathAbs[len(sharedDirAbs):]
		relativePath = strings.TrimLeft(relativePath, "/")
		fmt.Println(relativePath)
		sharedFile, err := os.Open(sharedFilePathAbs)
		if err != nil {
			log.Fatal(err)
		}
		sha256 := sha256File(sharedFile)
		fmt.Println(sha256)

		pack.Files = append(pack.Files, relativePath)
		pack.Hashes = append(pack.Hashes, sha256)
	}
}

func walker(path string, info os.FileInfo, err error) error {
    if err != nil {
        return err
    }

    if !info.IsDir() {
		targetFile, err := os.Open(path)
		if err != nil {
			log.Fatal(err)
		}
		defer targetFile.Close()

		if !strings.HasSuffix(path, ".pack") {
			return nil
		}

		buildPack(path, targetFile)
    }
    return nil
}

func main() {
	fileDir := "shared"
	var err error
	sharedDirAbs, err = filepath.Abs(fileDir)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("target:", sharedDirAbs)
	err = filepath.Walk(sharedDirAbs, walker)

	if err != nil {
	    log.Println(err)
	}
}
