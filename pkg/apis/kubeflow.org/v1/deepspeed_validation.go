// Copyright 2023 The Kubeflow Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"fmt"
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	apimachineryvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	commonv1 "github.com/kubeflow/common/pkg/apis/common/v1"
)

var (
	validCleanPolicies = sets.NewString(
		string(commonv1.CleanPodPolicyNone),
		string(commonv1.CleanPodPolicyRunning),
		string(commonv1.CleanPodPolicyAll))

	validRestartPolicies = sets.NewString(
		string(commonv1.RestartPolicyNever),
		string(commonv1.RestartPolicyOnFailure),
	)
)

func ValidateDeepSpeedJob(job *DeepSpeedJob) field.ErrorList {
	errs := validateDeepSpeedJobName(job)
	errs = append(errs, validateDeepSpeedJobSpec(&job.Spec, field.NewPath("spec"))...)
	return errs
}

func validateDeepSpeedJobName(job *DeepSpeedJob) field.ErrorList {
	var allErrs field.ErrorList
	var replicas int32 = 1
	if workerSpec := job.Spec.DeepSpeedReplicaSpecs[DeepSpeedReplicaTypeWorker]; workerSpec != nil {
		if workerSpec.Replicas != nil && *workerSpec.Replicas > 0 {
			replicas = *workerSpec.Replicas
		}
	}
	maximumPodHostname := fmt.Sprintf("%s-worker-%d", job.Name, replicas-1)
	if errs := apimachineryvalidation.IsDNS1035Label(maximumPodHostname); len(errs) > 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata").Child("name"), job.ObjectMeta.Name, fmt.Sprintf("will not able to create pod and service with invalid DNS label %q: %s", maximumPodHostname, strings.Join(errs, ", "))))
	}
	return allErrs
}

func validateDeepSpeedJobSpec(spec *DeepSpeedJobSpec, path *field.Path) field.ErrorList {
	errs := validateDeepSpeedReplicaSpecs(spec.DeepSpeedReplicaSpecs, path.Child("deepspeedReplicaSpecs"))
	if spec.SlotsPerWorker == nil {
		errs = append(errs, field.Required(path.Child("slotsPerWorker"), "must have number of slots per worker"))
	} else {
		errs = append(errs, apivalidation.ValidateNonnegativeField(int64(*spec.SlotsPerWorker), path.Child("slotsPerWorker"))...)
	}
	errs = append(errs, validateRunPolicy(&spec.RunPolicy, path.Child("runPolicy"))...)
	if spec.SSHAuthMountPath == "" {
		errs = append(errs, field.Required(path.Child("sshAuthMountPath"), "must have a mount path for SSH credentials"))
	}
	return errs
}

func validateRunPolicy(policy *commonv1.RunPolicy, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if policy.CleanPodPolicy == nil {
		errs = append(errs, field.Required(path.Child("cleanPodPolicy"), "must have clean Pod policy"))
	} else if !validCleanPolicies.Has(string(*policy.CleanPodPolicy)) {
		errs = append(errs, field.NotSupported(path.Child("cleanPodPolicy"), *policy.CleanPodPolicy, validCleanPolicies.List()))
	}
	// The remaining fields can be nil.
	if policy.TTLSecondsAfterFinished != nil {
		errs = append(errs, apivalidation.ValidateNonnegativeField(int64(*policy.TTLSecondsAfterFinished), path.Child("ttlSecondsAfterFinished"))...)
	}
	if policy.ActiveDeadlineSeconds != nil {
		errs = append(errs, apivalidation.ValidateNonnegativeField(*policy.ActiveDeadlineSeconds, path.Child("activeDeadlineSeconds"))...)
	}
	if policy.BackoffLimit != nil {
		errs = append(errs, apivalidation.ValidateNonnegativeField(int64(*policy.BackoffLimit), path.Child("backoffLimit"))...)
	}
	return errs
}

func validateDeepSpeedReplicaSpecs(replicaSpecs map[commonv1.ReplicaType]*commonv1.ReplicaSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if replicaSpecs == nil {
		errs = append(errs, field.Required(path, "must have replica specs"))
		return errs
	}
	errs = append(errs, validateLauncherReplicaSpec(replicaSpecs[DeepSpeedReplicaTypeLauncher], path.Key(string(DeepSpeedReplicaTypeLauncher)))...)
	errs = append(errs, validateWorkerReplicaSpec(replicaSpecs[DeepSpeedReplicaTypeWorker], path.Key(string(DeepSpeedReplicaTypeWorker)))...)
	return errs
}

func validateLauncherReplicaSpec(spec *commonv1.ReplicaSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if spec == nil {
		errs = append(errs, field.Required(path, fmt.Sprintf("must have %s replica spec", DeepSpeedReplicaTypeLauncher)))
		return errs
	}
	errs = append(errs, validateReplicaSpec(spec, path)...)
	if spec.Replicas != nil && *spec.Replicas != 1 {
		errs = append(errs, field.Invalid(path.Child("replicas"), *spec.Replicas, "must be 1"))
	}
	return errs
}

func validateWorkerReplicaSpec(spec *commonv1.ReplicaSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if spec == nil {
		return errs
	}
	errs = append(errs, validateReplicaSpec(spec, path)...)
	if spec.Replicas != nil && *spec.Replicas <= 0 {
		errs = append(errs, field.Invalid(path.Child("replicas"), *spec.Replicas, "must be greater than or equal to 1"))
	}
	return errs
}

func validateReplicaSpec(spec *commonv1.ReplicaSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if spec.Replicas == nil {
		errs = append(errs, field.Required(path.Child("replicas"), "must define number of replicas"))
	}
	if !validRestartPolicies.Has(string(spec.RestartPolicy)) {
		errs = append(errs, field.NotSupported(path.Child("restartPolicy"), spec.RestartPolicy, validRestartPolicies.List()))
	}
	if len(spec.Template.Spec.Containers) == 0 {
		errs = append(errs, field.Required(path.Child("template", "spec", "containers"), "must define at least one container"))
	}
	return errs
}