/*
Copyright 2024.

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

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("UpgradePlan Webhook", func() {
	Context("When creating UpgradePlans under Validating Webhook", func() {
		It("Should be denied if release version is not specified", func() {
			plan := &UpgradePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan1",
					Namespace: "default",
				},
			}

			err := k8sClient.Create(ctx, plan)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("release version is required")))
		})

		It("Should be denied if release version is not in semantic format", func() {
			plan := &UpgradePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan1",
					Namespace: "default",
				},
				Spec: UpgradePlanSpec{
					ReleaseVersion: "v1",
				},
			}

			err := k8sClient.Create(ctx, plan)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("'v1' is not a semantic version")))
		})
	})

	Context("When updating UpgradePlan under Validating Webhook", Ordered, func() {
		plan := &UpgradePlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "plan1",
				Namespace: "default",
			},
			Spec: UpgradePlanSpec{
				ReleaseVersion: "3.1.0",
			},
		}

		BeforeAll(func() {
			By("Creating the plan")
			Expect(k8sClient.Create(ctx, plan)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up status conditions")
			plan.Status.Conditions = nil
			Expect(k8sClient.Status().Update(ctx, plan)).To(Succeed())
		})

		It("Should be denied when an upgrade is pending", func() {
			condition := metav1.Condition{Type: KubernetesUpgradedCondition, Status: metav1.ConditionFalse, Reason: UpgradePending}

			meta.SetStatusCondition(&plan.Status.Conditions, condition)
			Expect(k8sClient.Status().Update(ctx, plan)).To(Succeed())

			err := k8sClient.Update(ctx, plan)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("upgrade plan cannot be edited while condition 'KubernetesUpgraded' is in 'Pending' state")))
		})

		It("Should be denied when an upgrade is in progress", func() {
			condition := metav1.Condition{Type: KubernetesUpgradedCondition, Status: metav1.ConditionFalse, Reason: UpgradeInProgress}

			meta.SetStatusCondition(&plan.Status.Conditions, condition)
			Expect(k8sClient.Status().Update(ctx, plan)).To(Succeed())

			err := k8sClient.Update(ctx, plan)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("upgrade plan cannot be edited while condition 'KubernetesUpgraded' is in 'InProgress' state")))
		})

		It("Should be denied when an upgrade is experiencing a transient error", func() {
			condition := metav1.Condition{Type: KubernetesUpgradedCondition, Status: metav1.ConditionFalse, Reason: UpgradeError}

			meta.SetStatusCondition(&plan.Status.Conditions, condition)
			Expect(k8sClient.Status().Update(ctx, plan)).To(Succeed())

			err := k8sClient.Update(ctx, plan)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("upgrade plan cannot be edited while condition 'KubernetesUpgraded' is in 'Error' state")))
		})

		It("Should be denied if release version is not specified", func() {
			plan.Spec.ReleaseVersion = ""

			err := k8sClient.Update(ctx, plan)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("release version is required")))
		})

		It("Should be denied if release version is not in semantic format", func() {
			plan.Spec.ReleaseVersion = "v1"

			err := k8sClient.Update(ctx, plan)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("'v1' is not a semantic version")))
		})

		It("Should be denied if the new release version is the same as the last applied one", func() {
			plan.Status.LastSuccessfulReleaseVersion = "3.1.0"
			Expect(k8sClient.Status().Update(ctx, plan)).To(Succeed())

			err := k8sClient.Update(ctx, plan)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("any edits over 'plan1' must come with an increment of the releaseVersion")))
		})

		It("Should be denied if the new release version is lesser than the last applied one", func() {
			plan.Spec.ReleaseVersion = "3.0.2"
			err := k8sClient.Update(ctx, plan)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("new releaseVersion must be greater than the currently applied one ('3.1.0')")))
		})

		It("Should pass if the new release version is greater than the last applied one", func() {
			plan.Spec.ReleaseVersion = "3.1.1"
			Expect(k8sClient.Update(ctx, plan)).To(Succeed())
		})
	})

})
