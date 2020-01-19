package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

func toReal(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Interface && !v.IsNil() {
		v = v.Elem()
	}
	return v
}

func filterEmpty(i interface{}) interface{} {
	v := reflect.ValueOf(i)

	switch v.Kind() {
	case reflect.Map:
		keys := v.MapKeys()
		for _, key := range keys {
			value := v.MapIndex(key)
			switch key.String() {
			case "pushed_at", "created_at", // timestamp are re-serialized as ints, github implements them as RFC3339
				"author_association", // association is not available in google's github library yet
				"open_issues", "node_id":
				v.SetMapIndex(key, reflect.Value{})
			default:
				if isEmptyValue(toReal(value)) {
					v.SetMapIndex(key, reflect.Value{})
				} else {
					v.SetMapIndex(key, reflect.ValueOf(filterEmpty(value.Interface())))
				}
			}
		}
	}

	return v.Interface()
}

func deserializeAnonymous(r io.Reader) interface{} {
	var d interface{}
	json.NewDecoder(r).Decode(&d)
	return filterEmpty(d)
}

func testEventParser(t *testing.T, path string) {
	t.Run(fmt.Sprintf("with event %s", path), func(t *testing.T) {
		os.Setenv("GITHUB_ACTIONS", "true")
		os.Setenv("GITHUB_HEAD_REF", "")
		os.Setenv("GITHUB_ACTOR", "tjamet")
		os.Setenv("GITHUB_ACTION", "run2")
		os.Setenv("GITHUB_REF", "refs/heads/master")
		os.Setenv("GITHUB_SHA", "d74fd518cf0410699c6b748924727686c1606d00")
		os.Setenv("GITHUB_EVENT_PATH", "/home/runner/work/_temp/_github_workflow/event.json")
		os.Setenv("GITHUB_BASE_REF", "")
		os.Setenv("GITHUB_REPOSITORY", "tjamet/actions-playground")
		os.Setenv("GITHUB_EVENT_NAME", "push")
		os.Setenv("GITHUB_WORKFLOW", "CI")
		os.Setenv("GITHUB_WORKSPACE", "/home/runner/work/actions-playground/actions-playground")
		os.Setenv("GITHUB_EVENT_PATH", path)
		e := ParseActionEnv()
		b := bytes.NewBuffer(nil)
		assert.NoError(t, json.NewEncoder(b).Encode(e.Payload))
		data, err := ioutil.ReadFile(path)
		assert.NoError(t, err)
		assert.Equal(t, deserializeAnonymous(bytes.NewReader(data)), deserializeAnonymous(b))
	})
}

func TestContext(t *testing.T) {
	testEventParser(t, "issues_event.json")
	testEventParser(t, "label_event.json")
	testEventParser(t, "milestone_event.json")
	testEventParser(t, "push_event.json")
}
