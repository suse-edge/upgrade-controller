package controller

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MergeHelmValues(t *testing.T) {
	installedValuesStr := `{
  "global": {
    "ironicIP": "147.28.230.5"
  },
  "metal3-ironic": {
    "service": {
      "type": "LoadBalancer"
    },
    "persistence": {
      "ironic": {
        "storageClass": "longhorn"
      }
    }
  }
}`

	installedValuesMap := map[string]interface{}{
		"global": map[string]interface{}{
			"ironicIP": "147.28.230.5",
		},
		"metal3-ironic": map[string]interface{}{
			"service": map[string]interface{}{
				"type": "LoadBalancer",
			},
			"persistence": map[string]interface{}{
				"ironic": map[string]interface{}{
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
		expectedValues  []byte
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
			expectedValues:  nil,
		},
		{
			name:            "Empty installed values string, non-empty release values",
			installedValues: "",
			releaseValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.5"}}`),
			},
			expectedValues: []byte(`{"global":{"ironicIP":"147.28.230.5"}}`),
		},
		{
			name:            "Non-empty installed values string, empty release values",
			installedValues: installedValuesStr,
			expectedValues:  []byte(`{"global":{"ironicIP":"147.28.230.5"},"metal3-ironic":{"persistence":{"ironic":{"storageClass":"longhorn"}},"service":{"type":"LoadBalancer"}}}`),
		},
		{
			name:            "Non-empty installed values string, non-empty release values",
			installedValues: installedValuesStr,
			releaseValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.105"}}`),
			},
			expectedValues: []byte(`{"global":{"ironicIP":"147.28.230.105"},"metal3-ironic":{"persistence":{"ironic":{"storageClass":"longhorn"}},"service":{"type":"LoadBalancer"}}}`),
		},
		{
			name:            "Empty installed values map, empty release values",
			installedValues: map[string]interface{}{},
			expectedValues:  nil,
		},
		{
			name:            "Empty installed values map, non-empty release values",
			installedValues: map[string]interface{}{},
			releaseValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.5"}}`),
			},
			expectedValues: []byte(`{"global":{"ironicIP":"147.28.230.5"}}`),
		},
		{
			name:            "Non-empty installed values map, empty release values",
			installedValues: installedValuesMap,
			expectedValues:  []byte(`{"global":{"ironicIP":"147.28.230.5"},"metal3-ironic":{"persistence":{"ironic":{"storageClass":"longhorn"}},"service":{"type":"LoadBalancer"}}}`),
		},
		{
			name:            "Non-empty installed values map, non-empty release values",
			installedValues: installedValuesMap,
			releaseValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.105"}}`),
			},
			expectedValues: []byte(`{"global":{"ironicIP":"147.28.230.105"},"metal3-ironic":{"persistence":{"ironic":{"storageClass":"longhorn"}},"service":{"type":"LoadBalancer"}}}`),
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
			expectedValues: []byte(`{"global":{"ironicIP":"147.28.230.105"},"metal3-ironic":{"persistence":{"ironic":{"storageClass":"local-path-provisioner"}},"service":{"type":"LoadBalancer"}}}`),
		},
		{
			name:            "Invalid installed values string",
			installedValues: "{",
			expectedErr:     "unmarshaling installed chart values: unexpected end of JSON input",
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
			} else {
				require.NoError(t, err)
				assert.Equal(t, string(test.expectedValues), string(values))
			}
		})
	}
}
