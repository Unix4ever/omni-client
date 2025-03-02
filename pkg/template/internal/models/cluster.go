// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-version"
	"github.com/siderolabs/gen/pair"

	"github.com/siderolabs/omni-client/pkg/constants"
	"github.com/siderolabs/omni-client/pkg/omni/resources"
	"github.com/siderolabs/omni-client/pkg/omni/resources/omni"
)

// KindCluster is Cluster model kind.
const KindCluster = "Cluster"

// Cluster is a top-level template object.
type Cluster struct {
	Meta `yaml:",inline"`

	// Name is the name of the cluster.
	Name string `yaml:"name"`

	// Labels are the user labels to apply to the cluster.
	Labels map[string]string `yaml:"labels"`

	// Kubernetes settings.
	Kubernetes KubernetesCluster `yaml:"kubernetes"`

	// Talos settings.
	Talos TalosCluster `yaml:"talos"`

	// Cluster-wide patches.
	Patches PatchList `yaml:"patches"`
}

// KubernetesCluster is a Kubernetes cluster settings.
type KubernetesCluster struct {
	// Version is the Kubernetes version.
	Version string `yaml:"version"`
}

// TalosCluster is a Talos cluster settings.
type TalosCluster struct {
	// Version is the Talos version.
	Version string `yaml:"version"`
}

// Validate the model.
func (cluster *Cluster) Validate() error {
	var multiErr error

	if cluster.Name == "" {
		multiErr = multierror.Append(multiErr, fmt.Errorf("name is required"))
	}

	for _, c := range cluster.Name {
		if !unicode.IsDigit(c) && !unicode.IsLetter(c) && c != '-' && c != '_' {
			multiErr = multierror.Append(multiErr, fmt.Errorf("name should only contain letters, digits, dashes and underscores"))

			break
		}
	}

	for labelKey := range cluster.Labels {
		if strings.HasPrefix(labelKey, omni.SystemLabelPrefix) {
			multiErr = multierror.Append(multiErr, fmt.Errorf("label %q is invalid: prefix %q is reserved for internal use", labelKey, omni.SystemLabelPrefix))
		}
	}

	multiErr = joinErrors(multiErr, cluster.Kubernetes.Validate(), cluster.Talos.Validate(), cluster.Patches.Validate())

	if multiErr != nil {
		return fmt.Errorf("error validating cluster %q: %w", cluster.Name, multiErr)
	}

	return nil
}

// Validate the model.
func (kubernetes *KubernetesCluster) Validate() error {
	var multiErr error

	if kubernetes.Version == "" {
		multiErr = multierror.Append(multiErr, fmt.Errorf("version is required"))
	} else if _, err := version.NewSemver(kubernetes.Version); err != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("version should be in semver format: %w", err))
	}

	if multiErr != nil {
		return fmt.Errorf("error validating Kubernetes version: %w", multiErr)
	}

	return nil
}

// Validate the model.
func (talos *TalosCluster) Validate() error {
	var multiErr error

	if talos.Version == "" {
		multiErr = multierror.Append(multiErr, fmt.Errorf("version is required"))
	} else {
		if _, err := version.NewSemver(talos.Version); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("version should be in semver format: %w", err))
		}

		if !strings.HasPrefix(talos.Version, "v") {
			multiErr = multierror.Append(multiErr, fmt.Errorf("version should start with 'v'"))
		}
	}

	if multiErr != nil {
		return fmt.Errorf("error validating Talos version: %w", multiErr)
	}

	return nil
}

// Translate into Omni resources.
func (cluster *Cluster) Translate(TranslateContext) ([]resource.Resource, error) {
	clusterResource := omni.NewCluster(resources.DefaultNamespace, cluster.Name)

	for key, value := range cluster.Labels {
		clusterResource.Metadata().Labels().Set(key, value)
	}

	clusterResource.TypedSpec().Value.KubernetesVersion = strings.TrimLeft(cluster.Kubernetes.Version, "v")
	clusterResource.TypedSpec().Value.TalosVersion = strings.TrimLeft(cluster.Talos.Version, "v")

	patches, err := cluster.Patches.Translate(fmt.Sprintf("cluster-%s", cluster.Name), constants.PatchBaseWeightCluster, pair.MakePair(omni.LabelCluster, cluster.Name))
	if err != nil {
		return nil, err
	}

	return append([]resource.Resource{clusterResource}, patches...), nil
}

func init() {
	register[Cluster](KindCluster)
}

func joinErrors(err error, errs ...error) error {
	for _, e := range errs {
		if e != nil {
			err = multierror.Append(err, e)
		}
	}

	return err
}
