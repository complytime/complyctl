/*
 Copyright 2024 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package extensions

import (
	"strings"

	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
)

// TrestleNameSpace is the generic namespace for trestle-defined property extensions.
const TrestleNameSpace = "https://oscal-compass.github.io/compliance-trestle/schemas/oscal"

// Below are defined oscal.Property names for compass-based extensions.
const (
	// RuleIdProp represents the property name for Rule ids.
	RuleIdProp = "Rule_Id"
	// RuleDescriptionProp represents the property name for Rule descriptions.
	RuleDescriptionProp = "Rule_Description"
	// CheckIdProp represents the property name for Check ids.
	CheckIdProp = "Check_Id"
	// CheckDescriptionProp represents the property name for Check descriptions.
	CheckDescriptionProp = "Check_Description"
	// ParameterIdProp represents the property name for Parameter ids.
	ParameterIdProp = "Parameter_Id"
	// ParameterDescriptionProp represents the property name for Parameter descriptions.
	ParameterDescriptionProp = "Parameter_Description"
	// ParameterDefaultProp represents the property name for Parameter default selected values.
	ParameterDefaultProp = "Parameter_Value_Default"
	// FrameworkProp represents the property name for the control source short name.
	FrameworkProp = "Framework_Short_Name"
	// TestParameterClass represents the property class for all test parameters
	// in OSCAL Activity types in Assessment Plans.
	TestParameterClass = "test-parameter"
	// AssessmentRuleIdProp represents the property name for a rule associated to an OSCAL
	// Observation.
	AssessmentRuleIdProp = "assessment-rule-id"
	// AssessmentCheckIdProp represents the property name for a check associated to an OSCAL
	// Observation.
	AssessmentCheckIdProp = "assessment-check-id"
)

type findOptions struct {
	class     string
	name      string
	namespace string
}

// FindOption define an option for searching oscal Property
// sets.
type FindOption func(opts *findOptions)

// WithName defines a FindOptions to search for properties
// with a given name.
func WithName(name string) FindOption {
	return func(opts *findOptions) {
		opts.name = name
	}
}

// WithClass defines a FindOptions to search for properties
// of a given class.
func WithClass(class string) FindOption {
	return func(opts *findOptions) {
		opts.class = class
	}
}

// WithNamespace defines a FindOptions to search for properties
// in a given namespace. The default is the TrestleNameSpace.
func WithNamespace(namespace string) FindOption {
	return func(opts *findOptions) {
		opts.namespace = namespace
	}
}

// FindAllProps returns all properties with the given options. By default, all properties with a Trestle
// namespace will be returned.
func FindAllProps(props []oscalTypes.Property, opts ...FindOption) []oscalTypes.Property {
	options := findOptions{
		namespace: TrestleNameSpace,
	}
	for _, opt := range opts {
		opt(&options)
	}

	var matchingProps []oscalTypes.Property
	for _, prop := range props {

		if strings.Contains(prop.Ns, options.namespace) {
			if options.name != "" && prop.Name != options.name {
				continue
			}
			if options.class != "" && prop.Class != options.class {
				continue
			}
			matchingProps = append(matchingProps, prop)
		}
	}
	return matchingProps
}

// GetTrestleProp returned  the first property matching the given name and a match is found.
// This function also implicitly checks that the property is a trestle-defined property in the namespace.
func GetTrestleProp(name string, props []oscalTypes.Property) (oscalTypes.Property, bool) {
	for _, prop := range props {
		if prop.Name == name && strings.Contains(prop.Ns, TrestleNameSpace) {
			return prop, true
		}
	}
	return oscalTypes.Property{}, false
}
