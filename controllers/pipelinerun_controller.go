/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PipelineRunReconciler reconciles a PipelineRun object
type PipelineRunReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns/status,verbs=get;update;patch

// Reconcile garbage collects old PipelineRuns
func (r *PipelineRunReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.Log.WithValues("pipelinerun", req.NamespacedName)

	// Remove five-day-old PipelineRuns
	pipelineRuns := &tektonv1beta1.PipelineRunList{}
	err := r.Client.List(ctx, pipelineRuns)
	if err != nil {
		logger.Error(err, "unable to list PipelineRuns")
	}
	fiveDaysAgo := time.Now().AddDate(0, 0, -5)
	for _, pr := range pipelineRuns.Items {
		if pr.CreationTimestamp.Time.Before(fiveDaysAgo) {
			err = r.Client.Delete(ctx, &pr)
			if err != nil {
				return ctrl.Result{}, err
			}
			logger.Info(fmt.Sprintf("deleted pipelienrun/%s", pr.Name))
		}
	}

	// Remove day-old PipelineRuns if they have a PVC attached
	pvcs := &v1.PersistentVolumeClaimList{}
	err = r.Client.List(ctx, pvcs)
	if err != nil {
		logger.Error(err, "unable to list PersistentVolumeClaims")
	}
	yesterday := time.Now().AddDate(0, 0, -1)
	for _, pvc := range pvcs.Items {
		for _, ownerRef := range pvc.OwnerReferences {
			if ownerRef.Kind == pipeline.PipelineRunControllerName {
				namespacedName := types.NamespacedName{
					Name:      ownerRef.Name,
					Namespace: pvc.Namespace,
				}
				pipelineRun := &tektonv1beta1.PipelineRun{}
				err = r.Client.Get(ctx, namespacedName, pipelineRun)
				if err != nil {
					return ctrl.Result{}, err
				}

				if pipelineRun.CreationTimestamp.Time.Before(yesterday) {
					err = r.Client.Delete(ctx, pipelineRun)
					if err != nil {
						return ctrl.Result{}, err
					}
					logger.Info(fmt.Sprintf("deleted pipelinrun/%s", pipelineRun.Name))
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller, with manager
func (r *PipelineRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tektonv1beta1.PipelineRun{}).
		Complete(r)
}
