package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

/*
	on start / refresh / monitor?
	walk directory
	load own packs

	advertise own packs

	other packs -> signatures in some recent time frame

	peers send signed packs
	client show packs with sig counts
	user selects a pack
	user requests pack
	downloads/party-line/channel is created if not present
	pieces come in
	client constructs chains
		map[hash of block] block
		as blocks are verified they are written to disk
*/

type DotPack struct {
	Name  string
	Files []string
}

type PackFileInfo struct {
	Name           string
	Hash           string
	FirstBlockHash string
	Size           int64
	BlockMap       map[string]*BlockInfo `json:"-"`
	Coverage       []uint64              `json:"-"`
	Path           string                `json:"-"`
}

const (
	AVAILABLE = iota
	ACTIVE
	COMPLETE
)

type Pack struct {
	Name  string
	Files []*PackFileInfo
	State int                  `json:"-"`
	Peers map[string]time.Time `json:"-"`
}

type BlockInfo struct {
	Index          uint64
	NextBlockHash  string
	LeftBlockHash  string
	RightBlockHash string
	DataHash       string
}

type Block struct {
	Index          uint64
	NextBlockHash  string
	LeftBlockHash  string
	RightBlockHash string
	Data           []byte
	DataHash       string
}

var sharedDir string
var fileMod map[string]time.Time

const BUFFER_SIZE = 10240

func initFiles() {
	if shareFlag != nil && *shareFlag != "" {
		sharedDir = *shareFlag
	} else {
		home, err := homedir.Dir()
		if err != nil {
			log.Fatal("could not get home dir")
		}
		sharedDir = filepath.Join(home, "party-line")
	}

	err := os.MkdirAll(sharedDir, 0700)
	if err != nil {
		log.Fatal("could not create shared dir")
	}

	fmt.Println("sharing", sharedDir)
	fileMod = make(map[string]time.Time)
}

type ByFileName []*PackFileInfo

func (packFileInfo ByFileName) Len() int {
	return len(packFileInfo)
}

func (packFileInfo ByFileName) Swap(i, j int) {
	packFileInfo[i], packFileInfo[j] = packFileInfo[j], packFileInfo[i]
}

func (packFileInfo ByFileName) Less(i, j int) bool {
	return packFileInfo[i].Name < packFileInfo[j].Name
}

// TODO: first pack received sets file names for ad
// an aggressive client could troll with file names
// I should fix this
func sha256Pack(pack *Pack) string {
	if pack == nil {
		return ""
	}

	sort.Sort(ByFileName(pack.Files))

	hash := sha256.New()
	hash.Write([]byte(pack.Name))
	for _, packFileInfo := range pack.Files {
		hash.Write([]byte(packFileInfo.Name))
		hash.Write([]byte(packFileInfo.Hash))
		hash.Write([]byte(packFileInfo.FirstBlockHash))
		hash.Write([]byte(strconv.FormatInt(packFileInfo.Size, 10)))
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func sha256File(targetFile *os.File) (string, error) {
	_, err := targetFile.Seek(0, 0)
	if err != nil {
		setStatus("error could not seek to start of file")
		log.Println(err)
		return "", err
	}

	h := sha256.New()
	if _, err := io.Copy(h, targetFile); err != nil {
		setStatus("error could not read file for hash")
		log.Println(err)
		return "", err
	}

	sha256 := fmt.Sprintf("%x", h.Sum(nil))
	return sha256, nil
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
	hash.Write([]byte(strconv.FormatUint(block.Index, 10)))
	hash.Write([]byte(block.NextBlockHash))
	hash.Write([]byte(block.DataHash))
	hash.Write(block.Data)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func unpackFile(targetFile *os.File) (*DotPack, error) {
	_, err := targetFile.Seek(0, 0)
	if err != nil {
		setStatus("error seek to start of file for unpack")
		log.Println(err)
		return nil, err
	}

	contents, err := ioutil.ReadAll(targetFile)
	if err != nil {
		setStatus("error could not read file for unpack")
		log.Println(err)
		return nil, err
	}

	dotPack := new(DotPack)
	err = json.Unmarshal(contents, dotPack)
	if err != nil {
		setStatus("error could not unmarshal json for unpack")
		log.Println(err)
		return nil, err
	}

	return dotPack, nil
}

func leftChild(i int64) int64 {
	return 2*i + 1
}

func rightChild(i int64) int64 {
	return 2*i + 2
}

func calculateChain(targetFile *os.File, size int64) (string, error) {
	if size < 0 {
		setStatus("error file size less than 0 (c'est une pipe?)")
		return "", errors.New("file size less than 0")
	}

	var prev *Block
	prev = nil

	blocks := make(map[string]*Block)
	lastBlockSize := size % BUFFER_SIZE
	index := size / BUFFER_SIZE

	_, err := targetFile.Seek(-lastBlockSize, 2)
	if err != nil {
		log.Println(err)
		setStatus("error seek failed in file")
		return "", errors.New("seek failed for file")
	}

	skips := make(map[int64]string)

	// read backward
	for index > -1 {
		buffer := make([]byte, BUFFER_SIZE) // 10 KiB
		bytesRead, err := targetFile.Read(buffer)
		if err != nil && err != io.EOF {
			log.Println(err)
			setStatus("error failed read")
			return "", errors.New("failed read of file")
		}

		sha256Buffer := sha256Bytes(buffer[:bytesRead])

		curr := new(Block)
		curr.Index = uint64(index)
		curr.Data = buffer[:bytesRead]
		curr.DataHash = sha256Buffer
		curr.NextBlockHash = sha256Block(prev)
		curr.LeftBlockHash = skips[leftChild(index)]
		curr.RightBlockHash = skips[rightChild(index)]

		blockHash := sha256Block(curr)
		blocks[blockHash] = curr

		prev = curr
		skips[index] = blockHash

		index--
		_, err = targetFile.Seek(-(int64(bytesRead) + BUFFER_SIZE), 1)
	}

	_, err = targetFile.Seek(0, 0)
	if err != nil {
		log.Println(err)
		setStatus("error could not seek to beginning of file")
		return "", errors.New("could not seek to beginning of file")
	}

	// test forward
	currBlockHash := sha256Block(prev)
	for index := 0; true; index++ {
		buffer := make([]byte, BUFFER_SIZE) // 10 KiB
		bytesRead, err := targetFile.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Println(err)
				setStatus("error could not read file (verify)")
				return "", errors.New("could not read file (verify)")
			}
			break
		}

		sha256Buffer := sha256Bytes(buffer[:bytesRead])

		curr := new(Block)
		curr.Index = uint64(index)
		curr.Data = buffer[:bytesRead]
		curr.DataHash = sha256Buffer
		curr.NextBlockHash = blocks[currBlockHash].NextBlockHash

		verifyBlockHash := sha256Block(curr)
		if verifyBlockHash != currBlockHash {
			log.Println("Bad hash at " + strconv.FormatInt(int64(index), 10))
			setStatus("error verify failed")
			return "", errors.New("verify failed")
		}

		currBlockHash = curr.NextBlockHash
	}

	return sha256Block(prev), nil
}

func fullCoverage(size int64) []uint64 {
	coverage := make([]uint64, 0)

	var curr uint64 = 0
	var i uint64 = 0
	for i = 0; i*BUFFER_SIZE < uint64(size); i++ {
		curr |= 1 << (i % 64)
		if (i+1)%64 == 0 {
			fmt.Println("adding", i)
			coverage = append(coverage, curr)
			curr = 0
		}
	}

	if curr != 0 {
		coverage = append(coverage, curr)
	}

	return coverage
}

func buildPack(partyId string, path string, targetFile *os.File) {
	partyDir := filepath.Join(sharedDir, partyId)
	partyDirAbs, err := filepath.Abs(partyDir)
	if err != nil {
		log.Println(err)
		setStatus("error could not get absolute path for party dir")
		return
	}

	if !strings.HasPrefix(partyDirAbs, sharedDir) {
		setStatus("error party dir traverses directories")
		return
	}

	pack := new(Pack)
	pack.Peers = make(map[string]time.Time)
	pack.Peers[peerSelf.Id()] = time.Now().UTC()
	pack.State = COMPLETE

	dotPack, err := unpackFile(targetFile)
	if err != nil {
		log.Println(err)
		setStatus("error unpacking pack file")
		return
	}

	if len(dotPack.Files) == 0 {
		setStatus("error no files in pack")
		return
	}

	pack.Name = dotPack.Name

	dirPath := filepath.Dir(path)
	for _, shortFilePath := range dotPack.Files {
		sharedFilePath := filepath.Join(dirPath, shortFilePath)
		sharedFilePathAbs, err := filepath.Abs(sharedFilePath)
		if err != nil {
			log.Println(err)
			setStatus("error could not get absolute path for file")
			return
		}

		if !strings.HasPrefix(sharedFilePathAbs, partyDir) {
			setStatus("error pack file outside of channel dir")
			return
		}

		relativePath := sharedFilePathAbs[len(partyDir):]
		relativePath = strings.TrimLeft(relativePath, "/")

		sharedFile, err := os.Open(sharedFilePathAbs)
		if err != nil {
			setStatus("error opening file in pack")
			log.Println(err)
			return
		}

		fileInfo, err := sharedFile.Stat()
		if err != nil {
			setStatus("error getting file info for file in pack")
			log.Println(err)
			return
		}

		fileHash, err := sha256File(sharedFile)
		if err != nil {
			log.Println(err)
			setStatus("error hashing shared file")
			return
		}

		sharedFileSize := fileInfo.Size()
		firstBlockHash, err := calculateChain(sharedFile, sharedFileSize)
		if err != nil {
			log.Println(err)
			setStatus(err.Error())
			return
		}

		packFileInfo := new(PackFileInfo)
		packFileInfo.Name = relativePath
		packFileInfo.Path = sharedFilePathAbs
		packFileInfo.Hash = fileHash
		packFileInfo.FirstBlockHash = firstBlockHash
		packFileInfo.Size = sharedFileSize
		packFileInfo.Coverage = fullCoverage(packFileInfo.Size)

		pack.Files = append(pack.Files, packFileInfo)
	}

	sort.Sort(ByFileName(pack.Files))
	packHash := sha256Pack(pack)

	parties[partyId].Packs[packHash] = pack
}

func walker(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	relPath := path[len(sharedDir):]
	relPath = strings.TrimLeft(relPath, "/")
	toks := strings.Split(relPath, "/")
	if len(toks) < 1 {
		setStatus("error bad path when walking")
		return nil
	}

	partyId := toks[0]
	if !info.IsDir() {
		targetFile, err := os.Open(path)
		if err != nil {
			setStatus("could not open files")
			return nil
		}

		defer targetFile.Close()

		if !strings.HasSuffix(path, ".pack") {
			return nil
		}

		buildPack(partyId, path, targetFile)
	}
	return nil
}

func resetPacks() {
	// check for new and changed packs
	for partyId, party := range parties {
		party.ClearPacks()
		targetDir := filepath.Join(sharedDir, partyId)

		_, err := os.Stat(targetDir)
		if err != nil {
			continue
		}

		err = filepath.Walk(targetDir, walker)
	}

	advertiseAll()
}
