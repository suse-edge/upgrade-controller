package controller

import (
	"testing"

	"gopkg.in/yaml.v3"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MergeMaps(t *testing.T) {
	tests := []struct {
		name   string
		m1     map[string]any
		m2     map[string]any
		result map[string]any
	}{
		{
			name:   "empty maps",
			m1:     map[string]any{},
			m2:     map[string]any{},
			result: map[string]any{},
		},
		{
			name: "non-empty first map, empty second map",
			m1: map[string]any{
				"a": 1,
			},
			m2: map[string]any{},
			result: map[string]any{
				"a": 1,
			},
		},
		{
			name: "empty first map, non-empty second map",
			m1:   map[string]any{},
			m2: map[string]any{
				"b": 5,
			},
			result: map[string]any{
				"b": 5,
			},
		},
		{
			name: "non-empty first map, non-empty second map",
			m1: map[string]any{
				"a": map[string]any{
					"a1": 1,
					"a2": 5,
				},
				"b": "five",
			},
			m2: map[string]any{
				"a": map[string]any{
					"a1": 100,
				},
				"c": 777,
			},
			result: map[string]any{
				"a": map[string]any{
					"a1": 100,
					"a2": 5,
				},
				"b": "five",
				"c": 777,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := mergeMaps(test.m1, test.m2)
			assert.Equal(t, test.result, result)
		})
	}
}

func Test_MergeHelmValues(t *testing.T) {
	installedValuesStr := `global:
  ironicIP: 147.28.230.5
metal3-ironic:
  service:
    type: LoadBalancer
  persistence:
    ironic:
      storageClass: longhorn
`

	installedValuesMap := map[string]any{
		"global": map[string]any{
			"ironicIP": "147.28.230.5",
		},
		"metal3-ironic": map[string]any{
			"service": map[string]any{
				"type": "LoadBalancer",
			},
			"persistence": map[string]any{
				"ironic": map[string]any{
					"storageClass": "longhorn",
				},
			},
		},
	}

	tests := []struct {
		name            string
		installedValues any
		releaseValues   *apiextensionsv1.JSON
		userValues      *apiextensionsv1.JSON
		expectedValues  map[string]any
		expectedErr     string
	}{
		{
			name:            "Invalid type of installed values",
			installedValues: 5,
			expectedErr:     "unexpected type int of installed values",
		},
		{
			name:            "Empty installed values string, empty release values",
			installedValues: "",
		},
		{
			name:            "Empty installed values string, non-empty release values",
			installedValues: "",
			releaseValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.5"}}`),
			},
			expectedValues: map[string]any{
				"global": map[string]any{
					"ironicIP": "147.28.230.5",
				},
			},
		},
		{
			name:            "Non-empty installed values string, empty release values",
			installedValues: installedValuesStr,
			expectedValues: map[string]any{
				"global": map[string]any{
					"ironicIP": "147.28.230.5",
				},
				"metal3-ironic": map[string]any{
					"service": map[string]any{
						"type": "LoadBalancer",
					},
					"persistence": map[string]any{
						"ironic": map[string]any{
							"storageClass": "longhorn",
						},
					},
				},
			},
		},
		{
			name:            "Non-empty installed values string, non-empty release values",
			installedValues: installedValuesStr,
			releaseValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.105"}}`),
			},
			expectedValues: map[string]any{
				"global": map[string]any{
					"ironicIP": "147.28.230.105",
				},
				"metal3-ironic": map[string]any{
					"service": map[string]any{
						"type": "LoadBalancer",
					},
					"persistence": map[string]any{
						"ironic": map[string]any{
							"storageClass": "longhorn",
						},
					},
				},
			},
		},
		{
			name:            "Empty installed values map, empty release values",
			installedValues: map[string]any{},
		},
		{
			name:            "Empty installed values map, non-empty release values",
			installedValues: map[string]any{},
			releaseValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.5"}}`),
			},
			expectedValues: map[string]any{
				"global": map[string]any{
					"ironicIP": "147.28.230.5",
				},
			},
		},
		{
			name:            "Non-empty installed values map, empty release values",
			installedValues: installedValuesMap,
			expectedValues: map[string]any{
				"global": map[string]any{
					"ironicIP": "147.28.230.5",
				},
				"metal3-ironic": map[string]any{
					"service": map[string]any{
						"type": "LoadBalancer",
					},
					"persistence": map[string]any{
						"ironic": map[string]any{
							"storageClass": "longhorn",
						},
					},
				},
			},
		},
		{
			name:            "Non-empty installed values map, non-empty release values",
			installedValues: installedValuesMap,
			releaseValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.105"}}`),
			},
			expectedValues: map[string]any{
				"global": map[string]any{
					"ironicIP": "147.28.230.105",
				},
				"metal3-ironic": map[string]any{
					"service": map[string]any{
						"type": "LoadBalancer",
					},
					"persistence": map[string]any{
						"ironic": map[string]any{
							"storageClass": "longhorn",
						},
					},
				},
			},
		},
		{
			name:            "Non-empty installed values map, non-empty release values, non-empty user values",
			installedValues: installedValuesMap,
			releaseValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.105"}}`),
			},
			userValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"metal3-ironic": {"persistence": {"ironic": {"storageClass": "local-path-provisioner"}}}}`),
			},
			expectedValues: map[string]any{
				"global": map[string]any{
					"ironicIP": "147.28.230.105",
				},
				"metal3-ironic": map[string]any{
					"service": map[string]any{
						"type": "LoadBalancer",
					},
					"persistence": map[string]any{
						"ironic": map[string]any{
							"storageClass": "local-path-provisioner",
						},
					},
				},
			},
		},
		{
			name:            "Invalid installed values string",
			installedValues: "{",
			expectedErr:     "unmarshaling installed chart values: yaml: line 1: did not find expected node content",
		},
		{
			name:            "Invalid release values",
			installedValues: "",
			releaseValues: &apiextensionsv1.JSON{
				Raw: []byte(`{`),
			},
			expectedErr: "unmarshaling additional release values: unexpected end of JSON input",
		},
		{
			name:            "Invalid user values",
			installedValues: "",
			userValues: &apiextensionsv1.JSON{
				Raw: []byte(`{`),
			},
			expectedErr: "unmarshaling additional user values: unexpected end of JSON input",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values, err := mergeHelmValues(test.installedValues, test.releaseValues, test.userValues)
			if test.expectedErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr)
				assert.Nil(t, values)
				return
			}

			require.NoError(t, err)

			if len(test.expectedValues) == 0 {
				assert.Nil(t, values)
			} else {
				b, err := yaml.Marshal(test.expectedValues)
				require.NoError(t, err)
				assert.Equal(t, string(b), string(values))
			}
		})
	}
}
