package evalreport

type Rollout struct {
	Flag       string
	Percentage int
	Salt       string
}

func NormalizePercentage(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func InRollout(rollout Rollout, identity string) bool {
	percentage := NormalizePercentage(rollout.Percentage)
	if percentage == 0 || identity == "" {
		return false
	}
	if percentage == 100 {
		return true
	}
	var sum uint64
	for _, char := range rollout.Salt + ":" + rollout.Flag + ":" + identity {
		sum = sum*131 + uint64(char)
	}
	return int(sum%100) < percentage
}

func Assign(rollout Rollout, identities []string) map[string]bool {
	result := make(map[string]bool, len(identities))
	for _, identity := range identities {
		result[identity] = InRollout(rollout, identity)
	}
	return result
}

func CountAssigned(assignments map[string]bool) (yes, no int) {
	for _, assigned := range assignments {
		if assigned {
			yes++
		} else {
			no++
		}
	}
	return yes, no
}
