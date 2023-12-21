package v1alpha1

type MergeMode string

const (
	// all types of sql joins
	LeftJoinUniq  MergeMode = "left-join-uniq"
	LeftJoin      MergeMode = "left-join"
	InnerJoinUniq MergeMode = "inner-join-uniq"
	InnerJoin     MergeMode = "inner-join"
	FullJoinUniq  MergeMode = "full-join-uniq"
	FullJoin      MergeMode = "full-join"

	// suffix
	UniqJoinSuffix = "-uniq"
)
