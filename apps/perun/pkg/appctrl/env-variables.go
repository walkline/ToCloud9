package appctrl

import "fmt"

func envMapToEnvSliceString(envs map[string]string) []string {
	res := make([]string, 0, len(envs))
	for k, v := range envs {
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}

	return res
}

func envSliceStringToEnvMap(envs []string) map[string]string {
	res := make(map[string]string, len(envs))
	for _, s := range envs {
		for j := 0; j < len(s); j++ {
			if s[j] == '=' {
				key := s[:j]
				value := s[j+1:]
				res[key] = value
			}
		}
	}

	return res
}

func mergeTwoEnvs(firstPriority, secondPriority map[string]string) map[string]string {
	res := make(map[string]string, len(firstPriority)+len(secondPriority))
	for k, v := range secondPriority {
		res[k] = v
	}

	for k, v := range firstPriority {
		res[k] = v
	}

	return res
}
