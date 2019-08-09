package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"testing"
)

func TestUnmarshal(t *testing.T) {
	filename := "testdata/ms.json"
	if testing.Verbose() {
		fmt.Println("reading", filename)
	}
	/*
		file, err := os.OpenFile(filename, os.O_RDONLY, 0644)
		if err != nil {
			t.Errorf(filename, err.Error())
		}
		defer file.Close()
	*/

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Errorf(filename, err.Error())
	}

	var rm *meshSrv
	rm = new(meshSrv)
	err = json.Unmarshal(data, rm)
	if err != nil {
		t.Error("json.Unmarshal:", err)
	}

	fmt.Printf("meshSrv: %v\n", rm)
}

/*
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	enc.Encode(GeoRegions)
*/
