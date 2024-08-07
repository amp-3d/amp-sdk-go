package amp

import (
	"bytes"
	fmt "fmt"
	io "io"
	"reflect"
	"testing"

	"github.com/amp-3d/amp-sdk-go/stdlib/tag"
)

func TestTxSerialize(t *testing.T) {
	// Test serialization of a simple TxMsg

	tx := NewTxMsg(true)
	tx.Status = OpStatus_Syncing
	tx.ContextID_0 = 888854513
	tx.ContextID_1 = 7777435
	tx.ContextID_2 = 77743773
	{
		op := TxOp{
			OpCode: TxOpCode_UpsertElement,
			CellID: tag.ID{4, 555, 666},
			AttrID: tag.ID{111312232, 22232334444},
			SI:     tag.ID{7383, 76549, 3773},
			EditID: tag.ID{123, 9999, 0},
		}
		tx.MarshalOp(&op, &Login{
			UserLabel: "lil turkey",
			UserUID: &Tag{
				Text: "cmdr5",
			},
			HostAddr: "batwing ave",
		})
		tx.DataStore = append(tx.DataStore, []byte("bytes not used but stored -- not normal!")...)

		op.SI[1] = 50454123
		op.EditID[1] += 37733773
		data := []byte("hello-world")
		for i := 0; i < 7; i++ {
			data = append(data, data...)
		}
		tx.MarshalOp(&op, &Login{
			UserUID: &Tag{
				Text: "anonymous",
			},
			HostAddr: string(data),
		})

		for i := 0; i < 5500; i++ {
			op.SI[0] = uint64(i)
			if i%5 == 0 {
				op.EditID[1] += 37
			}
			tx.MarshalOp(&op, &LoginResponse{
				HashResponse: append(data, fmt.Sprintf("-%d", i)...),
			})
		}

		op.SI[0] = 111111
		op.EditID[1] = 55445544
		op.OpCode = TxOpCode_DeleteElement
		tx.MarshalOpWithBuf(&op, nil)
	}

	var txBuf []byte
	tx.MarshalToBuffer(&txBuf)

	r := bufReader{
		buf: txBuf,
	}
	tx2, err := ReadTxMsg(&r)
	if err != nil {
		t.Errorf("ReadTxMsg failed: %v", err)
	}
	if tx2.TxInfo != tx.TxInfo {
		t.Errorf("ReadTxMsg failed: TxInfo mismatch")
	}
	if len(tx2.Ops) != len(tx.Ops) {
		t.Errorf("ReadTxMsg failed: TxInfo mismatch")
	}
	if !bytes.Equal(tx.DataStore, tx2.DataStore) {
		t.Errorf("ReadTxMsg failed: DataStore mismatch")
	}
	for i, op1 := range tx.Ops {
		op2 := tx2.Ops[i]

		if op1.OpCode != op2.OpCode || op1.CellID != op2.CellID || op1.AttrID != op2.AttrID || op1.SI != op2.SI || op1.DataOfs != op2.DataOfs || op1.DataLen != op2.DataLen {
			t.Errorf("ReadTxMsg failed: Op mismatch")
		}
	}
}

type bufReader struct {
	buf []byte
	pos int
}

func (r *bufReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}
	n = copy(p, r.buf[r.pos:])
	r.pos += n
	return n, nil
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	spec := reg.RegisterPrototype(AttrSpec.With("av"), &Tag{}, "")
	if spec.Canonic != AttrSpec.Canonic+".av.Tag" {
		t.Fatal("RegisterPrototype failed")
	}
	if spec.ID != (tag.Spec{}.With("amp.tag.attr.Tag.av")).ID {
		t.Fatalf("tag.FormSpec failed")
	}
	if spec.ID != AttrSpec.With("av").With("Tag").ID {
		t.Fatalf("tag.FormSpec failed")
	}
	if spec.ID.Base32Suffix() != "00rvscub" {
		t.Fatalf("unexpected spec.ID: %v", spec.ID)
	}
	if spec.ID.Base32() != "000000000000004w656q67gz3rm9s3vx00rvscub" {
		t.Errorf("tag.ID.Base32() failed: %v", spec.ID.Base32())
	}
	elem, err := reg.MakeValue(spec.ID)
	if err != nil {
		t.Fatalf("MakeValue failed: %v", err)
	}
	if reflect.TypeOf(elem) != reflect.TypeOf(&Tag{}) {
		t.Fatalf("MakeValue returned wrong type: %v", reflect.TypeOf(elem))
	}
}
