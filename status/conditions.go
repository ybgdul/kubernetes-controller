package status

import (

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetCondition(conditions *[]metav1.Condition, newCond metav1.Condition) bool { 
	if conditions == nil {
		conditions = &[]metav1.Condition{}
	}

	existingConds := *conditions
	for i, c := range existingConds { 
		if c.Type == newCond.Type { 
			if c.Status == newCond.Status && c.Reason == newCond.Reason && c.Message == newCond.Message { 
				return false
			}
			newCond.LastTransitionTime = metav1.Now()
			existingConds[i] = newCond
			return true
		}
	}

	newCond.LastTransitionTime = metav1.Now()
	*conditions = append(*conditions, newCond)
	return true
}

func IsConditionTrue(conditions []metav1.Condition, conditionType string) bool { 
	for _, c := range conditions { 
		if c.Type == conditionType {
			return true
		}
	}
	return false 
}