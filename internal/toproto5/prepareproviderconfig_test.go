package toproto5_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/internal/fwserver"
	"github.com/hashicorp/terraform-plugin-framework/internal/toproto5"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPrepareProviderConfigResponse(t *testing.T) {
	t.Parallel()

	testProto5Type := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"test_attribute": tftypes.String,
		},
	}

	testProto5Value := tftypes.NewValue(testProto5Type, map[string]tftypes.Value{
		"test_attribute": tftypes.NewValue(tftypes.String, "test-value"),
	})

	testProto5DynamicValue, err := tfprotov5.NewDynamicValue(testProto5Type, testProto5Value)

	if err != nil {
		t.Fatalf("unexpected error calling tfprotov5.NewDynamicValue(): %s", err)
	}

	testConfig := &tfsdk.Config{
		Raw: testProto5Value,
		Schema: schema.Schema{
			Attributes: map[string]schema.Attribute{
				"test_attribute": schema.StringAttribute{
					Required: true,
				},
			},
		},
	}

	testConfigInvalid := &tfsdk.Config{
		Raw: testProto5Value,
		Schema: schema.Schema{
			Attributes: map[string]schema.Attribute{
				"test_attribute": schema.BoolAttribute{
					Required: true,
				},
			},
		},
	}

	testCases := map[string]struct {
		input    *fwserver.ValidateProviderConfigResponse
		expected *tfprotov5.PrepareProviderConfigResponse
	}{
		"nil": {
			input:    nil,
			expected: nil,
		},
		"empty": {
			input:    &fwserver.ValidateProviderConfigResponse{},
			expected: &tfprotov5.PrepareProviderConfigResponse{},
		},
		"diagnostics": {
			input: &fwserver.ValidateProviderConfigResponse{
				Diagnostics: diag.Diagnostics{
					diag.NewWarningDiagnostic("test warning summary", "test warning details"),
					diag.NewErrorDiagnostic("test error summary", "test error details"),
				},
			},
			expected: &tfprotov5.PrepareProviderConfigResponse{
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
		"diagnostics-invalid-preparedconfig": {
			input: &fwserver.ValidateProviderConfigResponse{
				Diagnostics: diag.Diagnostics{
					diag.NewWarningDiagnostic("test warning summary", "test warning details"),
					diag.NewErrorDiagnostic("test error summary", "test error details"),
				},
				PreparedConfig: testConfigInvalid,
			},
			expected: &tfprotov5.PrepareProviderConfigResponse{
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
						Summary:  "Unable to Convert Configuration",
						Detail: "An unexpected error was encountered when converting the configuration to the protocol type. " +
							"This is always an issue in terraform-plugin-framework used to implement the provider and should be reported to the provider developers.\n\n" +
							"Please report this to the provider developer:\n\n" +
							"Unable to create DynamicValue: AttributeName(\"test_attribute\"): unexpected value type string, tftypes.Bool values must be of type bool",
					},
				},
			},
		},
		"preparedconfig": {
			input: &fwserver.ValidateProviderConfigResponse{
				PreparedConfig: testConfig,
			},
			expected: &tfprotov5.PrepareProviderConfigResponse{
				PreparedConfig: &testProto5DynamicValue,
			},
		},
	}

	for name, testCase := range testCases {
		name, testCase := name, testCase

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := toproto5.PrepareProviderConfigResponse(context.Background(), testCase.input)

			if diff := cmp.Diff(got, testCase.expected); diff != "" {
				t.Errorf("unexpected difference: %s", diff)
			}
		})
	}
}
