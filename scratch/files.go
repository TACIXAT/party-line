package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	Name  string
	Files []string
}

type Pack struct {
	Name            string
	Files           []string
	Hashes          []string
	LastBlockHashes []string
}

type File struct {
	Path     string
	Sha256   string
	Coverage []uint64
}

type Block struct {
	PrevBlockHash string
	Data          []byte
	DataHash      string
}

var sharedDirAbs string

func sha256File(targetFile *os.File) string {
	_, err := targetFile.Seek(0, 0)
	if err != nil {
		log.Fatal(err)
	}

	h := sha256.New()
	if _, err := io.Copy(h, targetFile); err != nil {
		log.Fatal(err)
	}

	sha256 := fmt.Sprintf("%x", h.Sum(nil))
	return sha256
}

func sha256Bytes(buffer []byte) string {
	sum := sha256.Sum256(buffer)
	return fmt.Sprintf("%x", sum)
}

func sha256Block(block *Block) string {
	if block == nil {
		return ""
	}

	hash := sha256.New()
	hash.Write([]byte(block.PrevBlockHash))
	hash.Write([]byte(block.DataHash))
	hash.Write(block.Data)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func unpackFile(targetFile *os.File) *PackFile {
	_, err := targetFile.Seek(0, 0)
	if err != nil {
		log.Fatal(err)
	}

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

func calculateChain(targetFile *os.File) string {
	_, err := targetFile.Seek(0, 0)
	if err != nil {
		log.Fatal(err)
	}

	BUFFER_SIZE := 10240

	var prev *Block
	prev = nil

	blocks := make(map[string]*Block)

	for {
		buffer := make([]byte, BUFFER_SIZE) // 10 KiB
		bytesRead, err := targetFile.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Fatal(err)
			}
			break
		}

		sha256Buffer := sha256Bytes(buffer[:bytesRead])

		curr := new(Block)
		curr.Data = buffer[:bytesRead]
		curr.DataHash = sha256Buffer
		curr.PrevBlockHash = sha256Block(prev)

		blockHash := sha256Block(curr)
		blocks[blockHash] = curr

		prev = curr
	}

	return sha256Block(prev)
}

func buildPack(path string, targetFile *os.File) {
	pack := new(Pack)

	packFile := unpackFile(targetFile)

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

		sharedFile, err := os.Open(sharedFilePathAbs)
		if err != nil {
			log.Fatal(err)
		}

		fileHash := sha256File(sharedFile)

		pack.Files = append(pack.Files, relativePath)
		pack.Hashes = append(pack.Hashes, fileHash)
		lastBlockHash := calculateChain(sharedFile)
		pack.LastBlockHashes = append(pack.LastBlockHashes, lastBlockHash)
	}

	fmt.Println("=== PACK ===\n")
	fmt.Println("Name:", pack.Name, "\n")
	for i := 0; i < len(pack.Files); i++ {
		fmt.Println("File:", pack.Files[i])
		fmt.Println("Hash:", pack.Hashes[i])
		fmt.Println("Last:", pack.LastBlockHashes[i])
		fmt.Println()
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
