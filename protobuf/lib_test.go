package stream

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"
)

func newTransaction(SenderKey []byte, Recipient []byte, Quantity []byte, data []byte, AvailGas uint64, GasPrice []byte) *TransactionMock {
	d := TransactionMock{
		Version:   100,
		AvailGas:  AvailGas,
		SenderKey: SenderKey,
		Recipient: Recipient,
		Quantity:  Quantity,
		GasPrice:  GasPrice,
		Data:      data,
	}

	return &d
}

type txJournal struct {
	Path string // Filesystem path to store the transactions at
	lock *sync.Mutex
	w    *os.File
}

func (t *txJournal) Lock() {
	t.lock.Lock()
}

func (t *txJournal) Unlock() {
	t.lock.Unlock()
}
func (t *txJournal) Write(p []byte) (n int, err error) {
	return t.w.Write(p)
}

func TestCommon(t *testing.T) {
	a := uint64(1234)
	s := []byte{10, 11, 12, 13, 14, 15}
	r := []byte{21, 22}
	q := []byte{30, 31, 32, 33}
	g := []byte{40, 41, 42, 43, 44}
	d := []byte{50, 51, 52, 53, 54, 55}

	t1 := newTransaction(s, r, q, d, a, g)
	m1, err := t1.Marshal()
	if err == nil {
		fmt.Println(m1)
	} else {
		t.Fatalf("Marshal error")
	}
	m2 := &TransactionMock{}
	m2.Unmarshal(m1)
	fmt.Println(m2.GetAvailGas(), m2.GetData(), m2.GetGasPrice(), m2.GetQuantity(), m2.GetRecipient())
	fmt.Println(m2.GetSenderKey(), m2.GetVersion(), m2.GasPrice)
}
func TestTransactionJournaling(t *testing.T) {
	// Create a temporary file for the journal
	dir, _ := os.Getwd()
	file, err := ioutil.TempFile(dir, "protobuf")
	if err != nil {
		t.Fatalf("failed to create temporary journal: %v", err)
	}
	journal := file.Name()
	fmt.Println(journal)
	// defer os.Remove(journal)
	// Skip the parsing if the journal file doens't exist at all
	if _, err := os.Stat(journal); os.IsNotExist(err) {
		fmt.Println(err)
	} else {
		fmt.Println("ok")
	}
	// Clean up the temporary file, we only need the path for now
	defer file.Close()
	defer os.Remove(journal)

	a := uint64(1234)
	s := []byte{10, 11, 12, 13, 14, 15}
	r := []byte{21, 22}
	q := []byte{30, 31, 32, 33}
	g := []byte{40, 41, 42, 43, 44}
	d := []byte{50, 51, 52, 53, 54, 55}

	t1 := newTransaction(s, r, q, d, a, g)

	// Skip the parsing if the journal file doens't exist at all
	if _, err := os.Stat(journal); os.IsNotExist(err) {
		fmt.Println(err)
	}
	// Open the journal for loading any past transactions
	input, err := os.OpenFile(journal, os.O_RDWR, 0777)
	if err != nil {
		fmt.Println(err)
	}

	defer input.Close()

	Write(input, t1)
	t1.Version = 200
	t1.Data = []byte{111, 222, 33, 44, 55, 58}
	Write(input, t1)
	input.Close()
	// Open the journal for loading any past transactions
	input, err = os.Open(journal)
	if err != nil {
		fmt.Println(err)
	}
	t2 := TransactionMock{}
	Read(input, &t2)
	fmt.Println(t2)
	Read(input, &t2)
	fmt.Println(t2)
}

type buffer struct {
	*bytes.Buffer
	closed bool
}

func newBuffer() *buffer {
	return &buffer{bytes.NewBuffer(nil), false}
}

func TestTransactionJournalingWithLock(t *testing.T) {
	// Create a temporary file for the journal
	dir, _ := os.Getwd()
	file, err := ioutil.TempFile(dir, "protobuf")
	if err != nil {
		t.Fatalf("failed to create temporary journal: %v", err)
	}
	journal := file.Name()
	fmt.Println(journal)
	// defer os.Remove(journal)
	// Skip the parsing if the journal file doens't exist at all
	if _, err := os.Stat(journal); os.IsNotExist(err) {
		fmt.Println(err)
	} else {
		fmt.Println("ok")
	}
	// Clean up the temporary file, we only need the path for now
	defer file.Close()
	defer os.Remove(journal)

	a := uint64(1234)
	s := []byte{10, 11, 12, 13, 14, 15}
	r := []byte{21, 22}
	q := []byte{30, 31, 32, 33}
	g := []byte{40, 41, 42, 43, 44}
	d := []byte{50, 51, 52, 53, 54, 55}

	t1 := newTransaction(s, r, q, d, a, g)

	// Skip the parsing if the journal file doens't exist at all
	if _, err := os.Stat(journal); os.IsNotExist(err) {
		fmt.Println(err)
	}
	// Open the journal for loading any past transactions
	input, err := os.OpenFile(journal, os.O_RDWR, 0777)
	if err != nil {
		fmt.Println(err)
	}

	defer input.Close()
	var lock sync.Mutex
	txjournal := new(txJournal)
	txjournal.Path = journal
	txjournal.w = input
	txjournal.lock = &lock

	WriteWithLock(txjournal, t1)
	t1.Version = 200
	t1.Data = []byte{111, 222, 33, 44, 55, 58}
	WriteWithLock(txjournal, t1)
	input.Close()
	// Open the journal for loading any past transactions
	input, err = os.Open(journal)
	if err != nil {
		fmt.Println(err)
	}
	t2 := TransactionMock{}
	Read(input, &t2)
	fmt.Println(t2)
	Read(input, &t2)
	fmt.Println(t2)
}
