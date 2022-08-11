package toproto5_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/internal/fwserver"
	"github.com/hashicorp/terraform-plugin-framework/internal/privatestate"
	"github.com/hashicorp/terraform-plugin-framework/internal/toproto5"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestImportResourceStateResponse(t *testing.T) {
	t.Parallel()

	testProto5Type := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"test_attribute": tftypes.String,
		},
	}

	testEmptyProto5Type := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{},
	}

	testProto5Value := tftypes.NewValue(testProto5Type, map[string]tftypes.Value{
		"test_attribute": tftypes.NewValue(tftypes.String, "test-value"),
	})

	testEmptyProto5Value := tftypes.NewValue(testEmptyProto5Type, map[string]tftypes.Value{})

	testProto5DynamicValue, err := tfprotov5.NewDynamicValue(testProto5Type, testProto5Value)

	if err != nil {
		t.Fatalf("unexpected error calling tfprotov5.NewDynamicValue(): %s", err)
	}

	testEmptyProto5DynamicValue, err := tfprotov5.NewDynamicValue(testEmptyProto5Type, testEmptyProto5Value)

	if err != nil {
		t.Fatalf("unexpected error calling tfprotov5.NewDynamicValue(): %s", err)
	}

	testState := tfsdk.State{
		Raw: testProto5Value,
		Schema: tfsdk.Schema{
			Attributes: map[string]tfsdk.Attribute{
				"test_attribute": {
					Required: true,
					Type:     types.StringType,
				},
			},
		},
	}

	testStateInvalid := tfsdk.State{
		Raw: testProto5Value,
		Schema: tfsdk.Schema{
			Attributes: map[string]tfsdk.Attribute{
				"test_attribute": {
					Required: true,
					Type:     types.BoolType,
				},
			},
		},
	}

	testEmptyState := tfsdk.State{
		Raw: testProto5Value,
		Schema: tfsdk.Schema{
			Attributes: map[string]tfsdk.Attribute{},
		},
	}

	testProviderKeyValue := privatestate.MustMarshalToJson(map[string][]byte{
		"providerKeyOne": []byte(`{"pKeyOne": {"k0": "zero", "k1": 1}}`),
	})

	testProviderData := privatestate.MustProviderData(context.Background(), testProviderKeyValue)

	testCases := map[string]struct {
		input    *fwserver.ImportResourceStateResponse
		expected *tfprotov5.ImportResourceStateResponse
	}{
		"nil": {
			input:    nil,
			expected: nil,
		},
		"empty": {
			input:    &fwserver.ImportResourceStateResponse{},
			expected: &tfprotov5.ImportResourceStateResponse{},
		},
		"diagnostics": {
			input: &fwserver.ImportResourceStateResponse{
				Diagnostics: diag.Diagnostics{
					diag.NewWarningDiagnostic("test warning summary", "test warning details"),
					diag.NewErrorDiagnostic("test error summary", "test error details"),
				},
			},
			expected: &tfprotov5.ImportResourceStateResponse{
				Diagnostics: []*tfprotov5.Diagnostic{
					{
						Severity: tfprotov5.DiagnosticSeverityWarning,
						Summary:  "test warning summary",
						Detail:   "test warning details",
					},
					{
						Severity: tfprotov5.DiagnosticSeverityError,
						Summary:  "test error summary",
						Detail:   "test error details",
					},
				},
			},
		},
		"diagnostics-invalid-newstate": {
			input: &fwserver.ImportResourceStateResponse{
				Diagnostics: diag.Diagnostics{
					diag.NewWarningDiagnostic("test warning summary", "test warning details"),
					diag.NewErrorDiagnostic("test error summary", "test error details"),
				},
				ImportedResources: []fwserver.ImportedResource{
					{
						State: testStateInvalid,
					},
				},
			},
			expected: &tfprotov5.ImportResourceStateResponse{
				Diagnostics: []*tfprotov5.Diagnostic{
					{
						Severity: tfprotov5.DiagnosticSeverityWarning,
						Summary:  "test warning summary",
						Detail:   "test warning details",
					},
					{
						Severity: tfprotov5.DiagnosticSeverityError,
						Summary:  "test error summary",
						Detail:   "test error details",
					},
					{
						Severity: tfprotov5.DiagnosticSeverityError,
						Summary:  "Unable to Convert State",
						Detail: "An unexpected error was encountered when converting the state to the protocol type. " +
							"This is always an issue in terraform-plugin-framework used to implement the provider and should be reported to the provider developers.\n\n" +
							"Please report this to the provider developer:\n\n" +
							"AttributeName(\"test_attribute\"): unexpected value type string, tftypes.Bool values must be of type bool",
					},
				},
			},
		},
		"newstate": {
			input: &fwserver.ImportResourceStateResponse{
				ImportedResources: []fwserver.ImportedResource{
					{
						State: testState,
					},
				},
			},
			expected: &tfprotov5.ImportResourceStateResponse{
				ImportedResources: []*tfprotov5.ImportedResource{
					{
						State: &testProto5DynamicValue,
					},
				},
			},
		},
		"private": {
			input: &fwserver.ImportResourceStateResponse{
				ImportedResources: []fwserver.ImportedResource{
					{
						State: testEmptyState,
						Private: &privatestate.Data{
							Framework: map[string][]byte{
								".frameworkKey": []byte(`{"fKeyOne": {"k0": "zero", "k1": 1}}`),
							},
							Provider: testProviderData,
						},
					},
				},
			},
			expected: &tfprotov5.ImportResourceStateResponse{
				ImportedResources: []*tfprotov5.ImportedResource{
					{
						State: &testEmptyProto5DynamicValue,
						Private: privatestate.MustMarshalToJson(map[string][]byte{
							".frameworkKey":  []byte(`{"fKeyOne": {"k0": "zero", "k1": 1}}`),
							"providerKeyOne": []byte(`{"pKeyOne": {"k0": "zero", "k1": 1}}`),
						}),
					},
				},
			},
		},
	}

	for name, testCase := range testCases {
		name, testCase := name, testCase

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := toproto5.ImportResourceStateResponse(context.Background(), testCase.input)

			if diff := cmp.Diff(got, testCase.expected); diff != "" {
				t.Errorf("unexpected difference: %s", diff)
			}
		})
	}
}
