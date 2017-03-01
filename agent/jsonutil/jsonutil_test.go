// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package jsonutil

import (
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ExampleMarshal() {
	type ColorGroup struct {
		ID     int
		Name   string
		Colors []string
	}
	group := ColorGroup{
		ID:     1,
		Name:   "Reds",
		Colors: []string{"Crimson", "Red", "Ruby", "Maroon"},
	}
	b, err := Marshal(group)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Println(b)
	// Output:
	// {"ID":1,"Name":"Reds","Colors":["Crimson","Red","Ruby","Maroon"]}
}

func ExampleRemarshal() {
	type ColorGroup struct {
		ID     int
		Name   string
		Colors []string
	}
	group := ColorGroup{
		ID:     1,
		Name:   "Reds",
		Colors: []string{"Crimson", "Red", "Ruby", "Maroon"},
	}

	var newGroup ColorGroup

	err := Remarshal(group, &newGroup)
	if err != nil {
		fmt.Println("error:", err)
	}

	out, err := Marshal(newGroup)
	if err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(out)
	// Output:
	// {"ID":1,"Name":"Reds","Colors":["Crimson","Red","Ruby","Maroon"]}
}

func ExampleIndent() {
	type Road struct {
		Name   string
		Number int
	}
	roads := []Road{
		{"Diamond Fork", 29},
		{"Sheep Creek", 51},
	}

	b, err := Marshal(roads)
	if err != nil {
		log.Fatal(err)
	}

	out := Indent(b)
	fmt.Println(out)
	// Output:
	// [
	//   {
	//     "Name": "Diamond Fork",
	//     "Number": 29
	//   },
	//   {
	//     "Name": "Sheep Creek",
	//     "Number": 51
	//   }
	// ]
}

func TestUnmarshalFile(t *testing.T) {
	filename := "rumpelstilzchen"
	var contents interface{}

	// missing file
	ioUtil = ioUtilStub{err: fmt.Errorf("some error")}
	err1 := UnmarshalFile(filename, &contents)
	assert.Error(t, err1, "expected readfile error")

	// non json content
	ioUtil = ioUtilStub{b: []byte("Sample text")}
	err2 := UnmarshalFile(filename, &contents)
	assert.Error(t, err2, "expected json parsing error")

	// valid json content
	ioUtil = ioUtilStub{b: []byte("{\"ID\":1,\"Name\":\"Reds\",\"Colors\":[\"Crimson\",\"Red\",\"Ruby\",\"Maroon\"]}")}
	err3 := UnmarshalFile(filename, &contents)
	assert.NoError(t, err3, "message should parse successfully")
}

func TestRemarshall(t *testing.T) {
	prop := make(map[string]string)
	prop["RunCommand"] = "echo"
	prop2 := make(map[string]string)
	prop2["command"] = "echo"
	type Property struct {
		RunCommand string
	}
	var newProp Property
	var newProp2 Property
	err := Remarshal(prop, &newProp)
	assert.NoError(t, err, "message should remarshall successfully")
	err = Remarshal(prop2, &newProp2)
	assert.NoError(t, err, "key mismatch should not report error")
	assert.Equal(t, Property{}, newProp2, "mismatched remarshall should return an empty object")
}

func TestUnmarshal(t *testing.T) {
	content1 := `{"parameter": "1"}`
	var dest interface{}

	err1 := Unmarshal(content1, &dest)
	assert.NoError(t, err1, "Message should parse correctly")

	content2 := `"Hello"`
	err2 := Unmarshal(content2, &dest)
	assert.NoError(t, err2, "This is not json format. Error expected")
}

// ioutil stub
type ioUtilStub struct {
	b   []byte
	err error
}

func (a ioUtilStub) ReadFile(filename string) ([]byte, error) {
	return a.b, a.err
}
