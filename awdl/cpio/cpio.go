package cpio

// BoDaay
// I had to implement cpio fully since I couldnt find a single reliable one to use
// I'll focus mostly on the basics to get the main project goopendrop to work
// The implementation has too many flaws, biggest it relies on memory 100%
// and I'm only supporting old ASCII format
import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
)

// Only ASCII Cpio being handled
const CpioMagic = "070707"

// the following I copied from same cpio archive sent by iphone using openairdrop python project
const Dummy_HeaderModeFolder = "040755"
const Dummy_HeaderModeFile = "100644"
const Dummy_HeaderDiv = "201406"
const Dummy_HeaderUid = "000755"
const Dummy_HeaderGid = "000755"
const Dummy_HeaderRDev = "000000"

func getIntToOctalStringPadded_width_6(number int64) string {
	return fmt.Sprintf("%06o", number)
}

func getIntToOctalStringPadded_width_11(number int64) string {
	return fmt.Sprintf("%011o", number)
}

type CpioArchive struct {
	archiveName      string
	archiveTotalSize uint64
	lastStartByte    uint64
	trailerAdded     bool
	basePath         string
	Entries          []CpioArchiveEntry
}

// Check IBM: https://www.ibm.com/docs/en/zos/2.2.0?topic=formats-cpio-format-cpio-archives
type CpioArchiveEntry struct {
	HeaderMagic    string `json:"H_Magic"`
	HeaderDev      string `json:"H_Dev"`
	HeaderIno      string `json:"H_Ino"`
	HeaderMode     string `json:"H_Mode"`
	HeaderUID      string `json:"H_UID"`
	HeaderGID      string `json:"H_GID"`
	HeaderNLinks   string `json:"H_NLinks"`
	HeaderRDEV     string `json:"H_RDev"`
	HeaderMTime    string `json:"H_MTime"`
	HeaderNameSize string `json:"H_FileNameSize"`
	HeaderFileSize string `json:"H_FileSize"`
	FileName       string `json:"FileName"`
	FileContent    []byte `json:"-"`
	FileSize       uint64 `json:"FileSize"`
	IsDirectory    bool   `json:"IsDirectory"`
	FileMime       string `json:"FileMime"`
	FileAppleUType string `json:"FileAppleUType"`
	TotalSize      uint64 `json:"TotalSize"`
	StartByteIndex uint64 `json:"StartByteIndex"`
}

func (c *CpioArchive) AddAllInFolder(foldername string, StripBase bool) error {
	fi, err := os.Stat(foldername)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s: is not a folder", foldername)
	}
	separator := string(os.PathSeparator)
	if !strings.HasSuffix(foldername, separator) {
		foldername += separator // add / at the end of path if not specified
	}
	if StripBase {
		c.basePath = foldername
	}
	err = filepath.Walk(foldername,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			err = c.addNewFileOrFolderEntry(path, StripBase)
			if err != nil {
				return err
			}
			return nil
		})

	if err != nil {
		return err
	}
	c.addTrailerEntry() // I will only support adding one folder and all of its content once
	return nil
}

func (c *CpioArchive) ExtractAllFiles(destination string) error {
	err := os.MkdirAll(destination, os.ModePerm)
	if err != nil {
		return err
	}
	for _, e := range c.Entries {
		if e.FileName == "TRAILER!!!" {
			break
		}
		if e.IsDirectory {
			err := os.MkdirAll(path.Join(destination, e.FileName), os.ModePerm)
			if err != nil {
				return err
			}
			continue
		}
		efile := filepath.Base(e.FileName)
		epath := filepath.Dir(e.FileName)
		if epath != "" {
			os.MkdirAll(path.Join(destination, epath), os.ModePerm)
		}
		err := os.WriteFile(path.Join(destination, epath, efile), e.FileContent, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}
func (c *CpioArchive) GetEntriesJSON() (string, error) {
	js, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return "", err
	}
	return string(js), nil
}
func (c *CpioArchive) AddEntry(e *CpioArchiveEntry) {
	e.StartByteIndex = c.lastStartByte + c.archiveTotalSize
	c.archiveTotalSize += e.TotalSize
	c.Entries = append(c.Entries, *e)
}

func (c *CpioArchive) addDotFolderEntry() {
	realPath := "."
	entry := CpioArchiveEntry{}
	totalBytes := 0
	entry.HeaderMagic = CpioMagic
	totalBytes += 6
	entry.HeaderDev = Dummy_HeaderDiv
	totalBytes += 6
	entry.HeaderIno = getIntToOctalStringPadded_width_6(int64(len(c.Entries)))[:6]
	totalBytes += 6
	entry.HeaderMode = Dummy_HeaderModeFolder
	// entry.HeaderMode = fmt.Sprintf("%04o", fi.Mode())
	totalBytes += 6
	entry.HeaderUID = Dummy_HeaderUid
	totalBytes += 6
	entry.HeaderGID = Dummy_HeaderGid
	totalBytes += 6
	entry.HeaderNLinks = "000003" // I'm not sure about this
	totalBytes += 6
	entry.HeaderRDEV = Dummy_HeaderRDev
	totalBytes += 6
	entry.HeaderMTime = getIntToOctalStringPadded_width_11(time.Now().Unix())
	totalBytes += 11
	entry.HeaderNameSize = getIntToOctalStringPadded_width_6(int64(len(realPath)) + 1) // +1 for null terminating character

	totalBytes += 6
	entry.HeaderFileSize = getIntToOctalStringPadded_width_11(0)
	totalBytes += 11
	// Update Fields we need not related to creating the archive
	entry.FileName = realPath

	totalBytes += len(realPath) + 1 //+1 for null terminating

	entry.FileSize = 0

	entry.IsDirectory = true
	c.AddEntry(&entry)
}
func (c *CpioArchive) AddFileByBytes(filename string, data []byte) error {
	if c.trailerAdded {
		return fmt.Errorf("trailer Entry already added, cannot add any new entries")
	}
	if len(c.Entries) == 0 {
		//better to add . folder entry
		c.addDotFolderEntry()
	}
	realPath := "./" + filename
	// fmode := fmt.Sprintf("%04o") // I'm just using iPhone Value, maybe it has not effect and I can use real value, no idea ^_^
	// log.Println(fsize, ftime, fmode)
	entry := CpioArchiveEntry{}
	totalBytes := 0
	entry.HeaderMagic = CpioMagic
	totalBytes += 6
	entry.HeaderDev = Dummy_HeaderDiv
	totalBytes += 6
	entry.HeaderIno = getIntToOctalStringPadded_width_6(int64(len(c.Entries)))[:6]
	totalBytes += 6
	entry.HeaderMode = Dummy_HeaderModeFile
	// entry.HeaderMode = fmt.Sprintf("%04o", fi.Mode())
	totalBytes += 6
	entry.HeaderUID = Dummy_HeaderUid
	totalBytes += 6
	entry.HeaderGID = Dummy_HeaderGid
	totalBytes += 6
	entry.HeaderNLinks = "000001" // I'm not sure about this
	totalBytes += 6
	entry.HeaderRDEV = Dummy_HeaderRDev
	totalBytes += 6
	entry.HeaderMTime = getIntToOctalStringPadded_width_11(time.Now().Unix())
	totalBytes += 11
	entry.HeaderNameSize = getIntToOctalStringPadded_width_6(int64(len(realPath)) + 1) // +1 for null terminating character

	totalBytes += 6
	entry.HeaderFileSize = getIntToOctalStringPadded_width_11(int64(len(data)))
	totalBytes += 11
	// Update Fields we need not related to creating the archive
	entry.FileName = realPath

	totalBytes += len(realPath) + 1 //+1 for null terminating

	entry.FileSize = uint64(len(data))

	mimetype.SetLimit(0)
	mtype := mimetype.Detect(data)
	entry.FileMime = mtype.String()
	entry.FileAppleUType = GetMatchingAppleUTypeIdentifier(mtype.String())
	entry.TotalSize = uint64(len(data)) + uint64(totalBytes)
	entry.FileContent = data
	c.AddEntry(&entry)
	return nil
}
func (c *CpioArchive) addNewFileOrFolderEntry(fileOrPath string, StripBase bool) error {
	if c.trailerAdded {
		return fmt.Errorf("trailer Entry already added, cannot add any new entries")
	}
	if c.basePath == "" {
		c.basePath = filepath.Dir(fileOrPath)
	}
	realPath := fileOrPath
	strippedPath := strings.Replace(fileOrPath, c.basePath, "."+string(os.PathSeparator), -1)
	//get stat of file
	fi, err := os.Stat(realPath)
	if err != nil {
		return err
	}

	fsize := fi.Size()
	ftime := fi.ModTime().Unix()
	// fmode := fmt.Sprintf("%04o") // I'm just using iPhone Value, maybe it has not effect and I can use real value, no idea ^_^
	// log.Println(fsize, ftime, fmode)
	entry := CpioArchiveEntry{}
	totalBytes := 0
	entry.HeaderMagic = CpioMagic
	totalBytes += 6
	entry.HeaderDev = Dummy_HeaderDiv
	totalBytes += 6
	entry.HeaderIno = getIntToOctalStringPadded_width_6(int64(len(c.Entries)))[:6]
	totalBytes += 6
	entry.HeaderMode = Dummy_HeaderModeFile
	// entry.HeaderMode = fmt.Sprintf("%04o", fi.Mode())
	totalBytes += 6
	entry.HeaderUID = Dummy_HeaderUid
	totalBytes += 6
	entry.HeaderGID = Dummy_HeaderGid
	totalBytes += 6
	entry.HeaderNLinks = "000001" // I'm not sure about this
	totalBytes += 6
	entry.HeaderRDEV = Dummy_HeaderRDev
	totalBytes += 6
	entry.HeaderMTime = getIntToOctalStringPadded_width_11(ftime)
	totalBytes += 11
	entry.HeaderNameSize = getIntToOctalStringPadded_width_6(int64(len(realPath)) + 1) // +1 for null terminating character
	if StripBase {
		entry.HeaderNameSize = getIntToOctalStringPadded_width_6(int64(len(strippedPath)) + 1) // +1 for null terminating character
	}
	totalBytes += 6
	entry.HeaderFileSize = getIntToOctalStringPadded_width_11(fsize)
	totalBytes += 11
	// Update Fields we need not related to creating the archive
	entry.FileName = fileOrPath
	if StripBase {
		entry.FileName = strippedPath
	}
	//FileName is part of header, and we need to include its size
	if StripBase {
		totalBytes += len(strippedPath) + 1 //+1 for null terminating
	} else {
		totalBytes += len(realPath) + 1 //+1 for null terminating
	}
	entry.FileSize = uint64(fi.Size())
	if !fi.IsDir() {
		mimetype.SetLimit(0)
		mtype, err := mimetype.DetectFile(realPath)
		if err == nil {
			entry.FileMime = mtype.String()
			// log.Println(mtype.String())
			entry.FileAppleUType = GetMatchingAppleUTypeIdentifier(mtype.String())
		} else {
			// log.Println(err)
			entry.FileAppleUType = GetMatchingAppleUTypeIdentifier("No Idea") //this will get: public.content
		}
		entry.TotalSize = uint64(fi.Size()) + uint64(totalBytes)
		//we need to read the binary data into FileContent
		entry.FileContent, err = os.ReadFile(realPath)
		if err != nil {
			return err
		}
	}

	// Update values if this is a folder
	if fi.IsDir() {
		entry.HeaderMode = Dummy_HeaderModeFolder
		entry.HeaderNLinks = "000003" // I'm not sure about this
		entry.HeaderFileSize = getIntToOctalStringPadded_width_11(0)
		entry.FileSize = 0
		entry.IsDirectory = true
		entry.TotalSize = uint64(totalBytes)
	}
	c.AddEntry(&entry)
	return nil
}
func (c *CpioArchive) WriteByteArchive(Gzipped bool) ([]byte, error) {
	//we need to add trailer entry
	if !c.trailerAdded {
		c.addTrailerEntry()
	}
	var buf bytes.Buffer
	var bufGZ bytes.Buffer
	var output io.Writer
	var gf *gzip.Writer
	var fw *bufio.Writer

	if Gzipped {
		gf = gzip.NewWriter(&bufGZ)
		fw = bufio.NewWriter(gf)
		output = fw
	} else {

		output = &buf
	}
	totalWritten := 0
	for _, e := range c.Entries {
		count, _ := output.Write([]byte(e.HeaderMagic))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderDev))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderIno))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderMode))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderUID))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderGID))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderNLinks))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderRDEV))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderMTime))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderNameSize))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderFileSize))
		totalWritten += count
		count, _ = output.Write([]byte(e.FileName))
		totalWritten += count
		count, _ = output.Write([]byte{0x00}) // null terminating file name
		totalWritten += count
		if !e.IsDirectory {
			count, _ = output.Write(e.FileContent)
			totalWritten += count
		}

	}
	// output.Write(make([]byte, 1024))
	if Gzipped {
		fw.Flush()
		// Close the gzip first.
		gf.Close()
		return bufGZ.Bytes(), nil
	}

	return buf.Bytes(), nil
}
func (c *CpioArchive) WriteArchive(destinationFile string, Gzipped bool) error {
	//we need to add trailer entry
	if !c.trailerAdded {
		c.addTrailerEntry()
	}
	var output io.Writer
	var gf *gzip.Writer
	var fw *bufio.Writer
	var f *os.File

	if Gzipped {
		f, err := os.OpenFile(destinationFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
		if err != nil {
			return err
		}
		gf = gzip.NewWriter(f)
		fw = bufio.NewWriter(gf)
		output = fw

	} else {
		f, err := os.Create(destinationFile)
		if err != nil {
			return err
		}
		output = f
	}
	defer f.Close()
	totalWritten := 0
	for _, e := range c.Entries {
		count, _ := output.Write([]byte(e.HeaderMagic))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderDev))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderIno))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderMode))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderUID))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderGID))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderNLinks))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderRDEV))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderMTime))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderNameSize))
		totalWritten += count
		count, _ = output.Write([]byte(e.HeaderFileSize))
		totalWritten += count
		count, _ = output.Write([]byte(e.FileName))
		totalWritten += count
		count, _ = output.Write([]byte{0x00}) // null terminating file name
		totalWritten += count
		if !e.IsDirectory {
			count, _ = output.Write(e.FileContent)
			totalWritten += count
		}

	}
	if Gzipped {
		fw.Flush()
		// Close the gzip first.
		gf.Close()
	}
	return nil
}
func (c *CpioArchive) addTrailerEntry() {
	entry := CpioArchiveEntry{}
	filename := "TRAILER!!!"
	entry.HeaderMagic = CpioMagic[:6]
	entry.HeaderDev = "000000"
	entry.HeaderIno = getIntToOctalStringPadded_width_6(int64(len(c.Entries)))[:6]
	entry.HeaderMode = "000000"
	entry.HeaderUID = "000000"
	entry.HeaderGID = "000000"
	entry.HeaderNLinks = "000000"
	entry.HeaderRDEV = "000000"
	entry.HeaderMTime = "00000000000"
	entry.HeaderNameSize = getIntToOctalStringPadded_width_6(int64(len(filename) + 1))[:6] // +1 for null or new line termination
	entry.HeaderFileSize = "00000000000"
	entry.FileName = filename

	c.Entries = append(c.Entries, entry)
	c.trailerAdded = true
}

func LoadCpioArchive(archiveFileName string) (*CpioArchive, error) {
	cparchive := CpioArchive{}

	fi, err := os.Stat(archiveFileName)
	if err != nil {
		return nil, err
	}
	cparchive.archiveName = archiveFileName
	//Parsing Entries
	//we will go entry by entry,

	afile, err := os.Open(archiveFileName)
	if err != nil {
		return nil, err
	}
	defer afile.Close()
	//check if archive is gzipped
	gzipMagic := make([]byte, 2)
	_, err = afile.Read(gzipMagic)
	if err != nil {
		return nil, err
	}
	TotalSizeOfData := fi.Size()
	var archivehandler io.Reader
	afile.Seek(0, 0)                                  // reset position
	if gzipMagic[0] == 0x1f && gzipMagic[1] == 0x8b { //if this is true, the file is gzippped
		compressedData, err := os.ReadFile(archiveFileName)
		if err != nil {
			return nil, err
		}
		b := bytes.NewBuffer(compressedData)
		var r io.Reader
		r, err = gzip.NewReader(b)
		if err != nil {
			return nil, err
		}
		var resB bytes.Buffer
		totalgzippedread, err := resB.ReadFrom(r)
		if err != nil {
			return nil, err
		}
		TotalSizeOfData = totalgzippedread
		archivehandler = bytes.NewReader(resB.Bytes())
	} else {
		archivehandler = afile
	}

	for i := 0; i < int(TotalSizeOfData); i++ {
		totalRead := 0
		entry := CpioArchiveEntry{}
		entry.StartByteIndex = uint64(i)
		entry.HeaderMagic = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderDev = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderIno = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderMode = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderUID = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderGID = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderNLinks = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderRDEV = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderMTime = string(readBytesFromFile(archivehandler, 11))
		totalRead += 11
		entry.HeaderNameSize = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderFileSize = string(readBytesFromFile(archivehandler, 11))
		totalRead += 11
		//now we actually read filename based on length, and content
		fNameLngth, err := strconv.ParseInt(entry.HeaderNameSize, 8, 64) //converting octal string to decimal
		if err != nil {
			return nil, err
		}
		entry.FileName = string(readBytesFromFile(archivehandler, int(fNameLngth)))[:fNameLngth-1] // no need for the null terminating
		totalRead += int(fNameLngth)
		fSizeLength, err := strconv.ParseInt(entry.HeaderFileSize, 8, 64)
		if err != nil {
			return nil, err
		}
		entry.FileSize = uint64(fSizeLength)
		if entry.FileSize > 0 { //this tell us right away that this is an actual file and not a directory
			entry.FileContent = readBytesFromFile(archivehandler, int(fSizeLength))
			mimetype.SetLimit(0)
			mtype := mimetype.Detect(entry.FileContent)
			entry.FileMime = mtype.String()
			entry.FileAppleUType = GetMatchingAppleUTypeIdentifier(entry.FileMime)
		}
		if entry.HeaderMode[1] == '4' { //not sure if this is correct, but if 2nd byte is 4, then it should be a folder
			entry.IsDirectory = true
		}
		totalRead += int(fSizeLength)
		i += totalRead
		entry.TotalSize = uint64(totalRead)
		cparchive.AddEntry(&entry)
		if strings.Contains(entry.FileName, "TRAILER!!!") {
			break
		}
	}
	return &cparchive, nil
}

func LoadCpioArchiveBytes(data []byte) (*CpioArchive, error) {
	cparchive := CpioArchive{}

	cparchive.archiveName = "donwloaded_cpio.cpio"
	var archivehandler io.Reader
	TotalSizeOfData := len(data)
	if data[0] == 0x1f && data[1] == 0x8b { //if this is true, the file is gzippped

		b := bytes.NewBuffer(data)
		var r io.Reader
		r, err := gzip.NewReader(b)
		if err != nil {
			return nil, err
		}
		var resB bytes.Buffer
		totalgzippedread, err := resB.ReadFrom(r)
		if err != nil {
			return nil, err
		}
		TotalSizeOfData = int(totalgzippedread)
		archivehandler = bytes.NewReader(resB.Bytes())
	} else {
		archivehandler = bytes.NewReader(data)
	}

	for i := 0; i < TotalSizeOfData; i++ {
		totalRead := 0
		entry := CpioArchiveEntry{}
		entry.StartByteIndex = uint64(i)
		entry.HeaderMagic = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderDev = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderIno = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderMode = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderUID = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderGID = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderNLinks = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderRDEV = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderMTime = string(readBytesFromFile(archivehandler, 11))
		totalRead += 11
		entry.HeaderNameSize = string(readBytesFromFile(archivehandler, 6))
		totalRead += 6
		entry.HeaderFileSize = string(readBytesFromFile(archivehandler, 11))
		totalRead += 11
		//now we actually read filename based on length, and content
		fNameLngth, err := strconv.ParseInt(entry.HeaderNameSize, 8, 64) //converting octal string to decimal
		if err != nil {
			return nil, err
		}
		entry.FileName = string(readBytesFromFile(archivehandler, int(fNameLngth)))[:fNameLngth-1] // no need for the null terminating

		totalRead += int(fNameLngth)
		fSizeLength, err := strconv.ParseInt(entry.HeaderFileSize, 8, 64)
		if err != nil {
			return nil, err
		}
		entry.FileSize = uint64(fSizeLength)
		if entry.FileSize > 0 { //this tell us right away that this is an actual file and not a directory
			entry.FileContent = readBytesFromFile(archivehandler, int(fSizeLength))
			mimetype.SetLimit(0)
			mtype := mimetype.Detect(entry.FileContent)
			entry.FileMime = mtype.String()
			entry.FileAppleUType = GetMatchingAppleUTypeIdentifier(entry.FileMime)
		}
		if entry.HeaderMode[1] == '4' { //not sure if this is correct, but if 2nd byte is 4, then it should be a folder
			entry.IsDirectory = true
		}
		totalRead += int(fSizeLength)
		i += totalRead
		entry.TotalSize = uint64(totalRead)
		cparchive.AddEntry(&entry)
		if strings.Contains(entry.FileName, "TRAILER!!!") {

			break
		}

	}
	return &cparchive, nil
}
func readBytesFromFile(handler io.Reader, length int) []byte {
	temp := make([]byte, length)
	handler.Read(temp) //maybe we need some sort of error handling, too lazy to do it

	return temp
}

// Below is just for Airdrop and Apple UType detection

// this is all baed on opendrop project and additional added from: https://developer.apple.com/documentation/uniformtypeidentifiers/uttype
func GetMatchingAppleUTypeIdentifier(mimeType string) string {
	result := "public.content"
	if strings.Contains(mimeType, "image/") {
		result = "public.image"
		if strings.Contains(mimeType, "jpg") || strings.Contains(mimeType, "jpeg") {
			result = "public.jpeg"
		} else if strings.Contains(mimeType, "jp2") {
			result = "public.jpeg-2000"
		} else if strings.Contains(mimeType, "gif") {
			result = "com.compuserve.gif"
		} else if strings.Contains(mimeType, "png") {
			result = "public.png"
		} else if strings.Contains(mimeType, "tiff") {
			result = "public.tiff"
		} else if strings.Contains(mimeType, "svg") {
			result = "public.svg-image"
		} else if strings.Contains(mimeType, "bmp") {
			result = "com.microsoft.bmp"
		} else if strings.Contains(mimeType, "raw") {
			result = "public.camera-raw-image"
		} else if strings.Contains(mimeType, "webp") {
			result = "org.webmproject.webp"
		}
	} else if strings.Contains(mimeType, "audio/") {
		result = "public.audio"
		if strings.Contains(mimeType, "mpeg") {
			result = "public.mp3"
		} else if strings.Contains(mimeType, "aiff") {
			result = "public.aiff-audio"
		} else if strings.Contains(mimeType, "wav") {
			result = "com.microsoft.waveform-audio"
		} else if strings.Contains(mimeType, "midi") {
			result = "public.midi-audio"
		}
	} else if strings.Contains(mimeType, "video/") {
		result = "public.video"
		if strings.Contains(mimeType, "quicktime") {
			result = "com.apple.quicktime-movie"
		} else if strings.Contains(mimeType, "mpeg") {
			result = "public.mpeg"
		} else if strings.Contains(mimeType, "mp4") {
			result = "public.mpeg-4"
		}
	} else if strings.Contains(mimeType, "application/") {
		result = "public.data"
		if strings.Contains(mimeType, "gzip") {
			result = "org.gnu.gnu-zip-archive"
		} else if strings.Contains(mimeType, "zip") {
			result = "org.zip-archive"
		} else if strings.Contains(mimeType, "pdf") {
			result = "com.adobe.pdf"
		} else if strings.Contains(mimeType, "x-bzip2") {
			result = "public.bzip2-archive"
		} else if strings.Contains(mimeType, "spreadsheet") {
			result = "org.openxmlformats.spreadsheetml.sheet"
		} else if strings.Contains(mimeType, "word") {
			result = "org.openxmlformats.wordprocessingml.document"
		} else if strings.Contains(mimeType, "epub") {
			result = "org.idpf.epub-container"
		} else if strings.Contains(mimeType, "presentation") {
			result = "org.openxmlformats.presentationml.presentation"
		} else if strings.Contains(mimeType, "json") {
			result = "public.json"
		}

	} else if strings.Contains(mimeType, "text/") {
		if strings.Contains(mimeType, "plain") {
			result = "public.text"
		} else if strings.Contains(mimeType, "html") {
			result = "public.html"
		} else if strings.Contains(mimeType, "xml") {
			result = "public.xml"
		} else if strings.Contains(mimeType, "rtf") {
			result = "public.rtf"
		} else if strings.Contains(mimeType, "csv") {
			result = "public.comma-separated-values-text"
		} else if strings.Contains(mimeType, "tab-separated-values") {
			result = "public.tab-separated-values-text"
		} else if strings.Contains(mimeType, "vcard") {
			result = "public.vcard"
		}
	}

	return result
}
