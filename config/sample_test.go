package config

import (
	"bytes"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func TestSampleConfigContainsEveryYAMLOption(t *testing.T) {
	contents, err := os.ReadFile("../tdns.yaml")
	if err != nil {
		t.Fatalf("read sample configuration: %v", err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	if err := decoder.Decode(&Config{}); err != nil {
		t.Fatalf("sample configuration contains an unsupported option: %v", err)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(contents, &document); err != nil {
		t.Fatalf("parse sample configuration: %v", err)
	}
	if len(document.Content) != 1 {
		t.Fatalf("sample configuration has %d root nodes, want 1", len(document.Content))
	}

	missing := missingYAMLOptions(reflect.TypeOf(Config{}), document.Content[0], "")
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("sample configuration is missing options: %s", strings.Join(missing, ", "))
	}
}

func TestYAMLAndMapstructureTagsMatch(t *testing.T) {
	mismatches := mismatchedConfigTags(reflect.TypeOf(Config{}), "")
	if len(mismatches) > 0 {
		sort.Strings(mismatches)
		t.Fatalf("configuration tags differ: %s", strings.Join(mismatches, ", "))
	}
}

func TestSampleConfigLoadsThroughViper(t *testing.T) {
	v := viper.New()
	v.SetConfigFile("../tdns.yaml")
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("read sample configuration with Viper: %v", err)
	}

	var loaded Config
	if err := v.Unmarshal(&loaded); err != nil {
		t.Fatalf("unmarshal sample configuration with Viper: %v", err)
	}
	if want := []string{"domain.tld", "facebook.com"}; !reflect.DeepEqual(loaded.Blacklist.Excludes, want) {
		t.Fatalf("blacklist.excludes = %v, want %v", loaded.Blacklist.Excludes, want)
	}
}

func mismatchedConfigTags(structType reflect.Type, prefix string) []string {
	mismatches := []string{}
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}

		yamlName := strings.Split(field.Tag.Get("yaml"), ",")[0]
		mapstructureName := strings.Split(field.Tag.Get("mapstructure"), ",")[0]
		if yamlName == "-" && mapstructureName == "-" {
			continue
		}
		if yamlName != mapstructureName {
			mismatches = append(mismatches, prefix+field.Name)
		}
		if field.Type.Kind() == reflect.Struct {
			mismatches = append(mismatches, mismatchedConfigTags(field.Type, prefix+field.Name+".")...)
		}
	}
	return mismatches
}

func missingYAMLOptions(structType reflect.Type, mapping *yaml.Node, prefix string) []string {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return []string{strings.TrimSuffix(prefix, ".")}
	}

	missing := []string{}
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}

		name := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(field.Name)
		}

		value := mappingValue(mapping, name)
		path := prefix + name
		if value == nil {
			missing = append(missing, path)
			continue
		}
		if field.Type.Kind() == reflect.Struct {
			missing = append(missing, missingYAMLOptions(field.Type, value, path+".")...)
		}
	}
	return missing
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}
