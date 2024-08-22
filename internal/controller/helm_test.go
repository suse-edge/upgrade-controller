package controller

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MergeHelmValues(t *testing.T) {
	tests := []struct {
		name            string
		installedValues any
		newValues       *apiextensionsv1.JSON
		expectedValues  []byte
		expectedErr     string
	}{
		{
			name:            "Invalid type of installed values",
			installedValues: 5,
			expectedErr:     "unexpected type int of installed values",
		},
		{
			name:            "Empty installed values string, empty new values",
			installedValues: "",
			expectedValues:  nil,
		},
		{
			name:            "Empty installed values string, non-empty new values",
			installedValues: "",
			newValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.5"}}`),
			},
			expectedValues: []byte(`{"global":{"ironicIP":"147.28.230.5"}}`),
		},
		{
			name:            "Non-empty installed values string, empty new values",
			installedValues: `{"global":{"ironicIP":"147.28.230.5"},"metal3-mariadb":{"persistence":{"storageClass":"longhorn"}}}`,
			expectedValues:  []byte(`{"global":{"ironicIP":"147.28.230.5"},"metal3-mariadb":{"persistence":{"storageClass":"longhorn"}}}`),
		},
		{
			name:            "Non-empty installed values string, non-empty new values",
			installedValues: `{"global":{"ironicIP":"147.28.230.5"},"metal3-mariadb":{"persistence":{"storageClass":"longhorn"}}}`,
			newValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.105"}}`),
			},
			expectedValues: []byte(`{"global":{"ironicIP":"147.28.230.105"},"metal3-mariadb":{"persistence":{"storageClass":"longhorn"}}}`),
		},
		{
			name:            "Empty installed values map, empty new values",
			installedValues: map[string]interface{}{},
			expectedValues:  nil,
		},
		{
			name:            "Empty installed values map, non-empty new values",
			installedValues: map[string]interface{}{},
			newValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.5"}}`),
			},
			expectedValues: []byte(`{"global":{"ironicIP":"147.28.230.5"}}`),
		},
		{
			name: "Non-empty installed values map, empty new values",
			installedValues: map[string]interface{}{
				"global": map[string]interface{}{
					"ironicIP": "147.28.230.5",
				},
				"metal3-mariadb": map[string]interface{}{
					"persistence": map[string]string{
						"storageClass": "longhorn",
					},
				},
			},
			expectedValues: []byte(`{"global":{"ironicIP":"147.28.230.5"},"metal3-mariadb":{"persistence":{"storageClass":"longhorn"}}}`),
		},
		{
			name: "Non-empty installed values map, non-empty new values",
			installedValues: map[string]interface{}{
				"global": map[string]interface{}{
					"ironicIP": "147.28.230.5",
				},
				"metal3-mariadb": map[string]interface{}{
					"persistence": map[string]string{
						"storageClass": "longhorn",
					},
				},
			},
			newValues: &apiextensionsv1.JSON{
				Raw: []byte(`{"global": {"ironicIP": "147.28.230.105"}}`),
			},
			expectedValues: []byte(`{"global":{"ironicIP":"147.28.230.105"},"metal3-mariadb":{"persistence":{"storageClass":"longhorn"}}}`),
		},
		{
			name:            "Invalid installed values string",
			installedValues: "{",
			expectedErr:     "unmarshaling installed chart values: unexpected end of JSON input",
		},
		{
			name:            "Invalid new values",
			installedValues: "",
			newValues: &apiextensionsv1.JSON{
				Raw: []byte(`{`),
			},
			expectedErr: "unmarshaling additional chart values: unexpected end of JSON input",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values, err := mergeHelmValues(test.installedValues, test.newValues)
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
