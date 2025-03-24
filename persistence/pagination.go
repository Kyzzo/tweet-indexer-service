package persistence

type Pagination[T any] struct {
	Page       int64 `json:"page"`
	Size       int64 `json:"size"`
	Result     T     `json:"result"`
	TotalCount int64 `json:"total_count"`
}

func NewPagination[T any](page int64, size int64, res T, totalCount int64) *Pagination[T] {
	return &Pagination[T]{
		Page:       page,
		Size:       size,
		TotalCount: totalCount,
		Result:     res,
	}
}
