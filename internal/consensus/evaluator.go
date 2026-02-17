package consensus

// EvaluateQuorum determines if enough nodes agree on a failure.
func EvaluateQuorum(quorumType string, quorumN int, failCount int, totalCount int) bool {
	switch quorumType {
	case "majority":
		return failCount > totalCount/2
	case "n_of_m":
		return failCount >= quorumN
	default:
		return failCount > totalCount/2
	}
}
