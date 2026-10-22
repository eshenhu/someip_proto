package semantic

import (
	"fmt"
	"math/big"

	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (a *payloadAnalyzer) validateFieldMetadata(
	field protoreflect.FieldDescriptor,
	options *optionsv1alpha1.FieldOptionsSpec,
	location descriptor.Location,
) {
	if options.GetString_() != nil && field.Kind() != protoreflect.StringKind {
		a.normalizer.diagnostics.error(ruleType, location, fmt.Sprintf("non-string field %s cannot declare string metadata", field.FullName()))
	}
	if options.GetBitfield() != nil && field.Kind() != protoreflect.Uint32Kind {
		a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("field %s can declare bitfield metadata only for uint32", field.FullName()))
	}
	if options.GetScalarRepresentation() != optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSPECIFIED &&
		(field.Kind() == protoreflect.StringKind || field.Kind() == protoreflect.BytesKind || field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind) {
		a.normalizer.diagnostics.error(ruleType, location, fmt.Sprintf("field %s cannot declare a scalar representation for protobuf kind %s", field.FullName(), field.Kind()))
	}
	a.validateValueDomain(field, options.GetDomain(), location)
}

func (a *payloadAnalyzer) validateValueDomain(
	field protoreflect.FieldDescriptor,
	domain *optionsv1alpha1.ValueDomain,
	location descriptor.Location,
) {
	if domain == nil {
		return
	}
	if !supportsValueDomain(field.Kind()) {
		a.normalizer.diagnostics.error(ruleType, location, fmt.Sprintf("field %s cannot declare a numeric value domain", field.FullName()))
		return
	}
	validScale := validateRational(a, location, field.FullName(), "scale", domain.GetScale(), false)
	validMinimum := validateRational(a, location, field.FullName(), "minimum", domain.GetMinimum(), false)
	validMaximum := validateRational(a, location, field.FullName(), "maximum", domain.GetMaximum(), false)
	validateRational(a, location, field.FullName(), "offset", domain.GetOffset(), false)
	if validScale && domain.GetScale() != nil && domain.GetScale().GetNumerator() == 0 {
		a.normalizer.diagnostics.error(ruleType, location, fmt.Sprintf("field %s value-domain scale must not be zero", field.FullName()))
	}
	if validMinimum && validMaximum && domain.GetMinimum() != nil && domain.GetMaximum() != nil && compareRational(domain.GetMinimum(), domain.GetMaximum()) > 0 {
		a.normalizer.diagnostics.error(ruleType, location, fmt.Sprintf("field %s value-domain minimum must not exceed maximum", field.FullName()))
	}
}

func validateRational(
	analyzer *payloadAnalyzer,
	location descriptor.Location,
	fieldName protoreflect.FullName,
	component string,
	value *optionsv1alpha1.Rational,
	requireValue bool,
) bool {
	if value == nil {
		if requireValue {
			analyzer.normalizer.diagnostics.error(ruleType, location, fmt.Sprintf("field %s value-domain %s is required when a domain is declared", fieldName, component))
		}
		return !requireValue
	}
	if value.GetDenominator() == 0 {
		analyzer.normalizer.diagnostics.error(ruleType, location, fmt.Sprintf("field %s value-domain %s must have a nonzero denominator", fieldName, component))
		return false
	}
	return true
}

func compareRational(left, right *optionsv1alpha1.Rational) int {
	leftNumerator := big.NewInt(left.GetNumerator())
	leftNumerator.Mul(leftNumerator, new(big.Int).SetUint64(right.GetDenominator()))
	rightNumerator := big.NewInt(right.GetNumerator())
	rightNumerator.Mul(rightNumerator, new(big.Int).SetUint64(left.GetDenominator()))
	return leftNumerator.Cmp(rightNumerator)
}

func supportsValueDomain(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Int32Kind,
		protoreflect.Int64Kind,
		protoreflect.Uint32Kind,
		protoreflect.Uint64Kind,
		protoreflect.FloatKind,
		protoreflect.DoubleKind,
		protoreflect.EnumKind:
		return true
	default:
		return false
	}
}
