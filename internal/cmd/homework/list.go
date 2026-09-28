package homework

import (
	"context"
	"net/url"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/cmd/youngutil"
	"github.com/Life-USTC/CLI/internal/openapi"
)

// Local homework filters and picking operate on the complete server collection.
func fetchAllHomeworks(ctx context.Context, client *api.TypedClient, sectionID *int64, includeDeleted *openapi.CommunitySectionHomeworkListParamsIncludeDeleted) (any, error) {
	path := "/api/workspace/homeworks"
	if sectionID != nil {
		path = "/api/community/section-homeworks"
	}
	return youngutil.FetchAllPages(ctx, func(ctx context.Context, query url.Values) (any, error) {
		page, err := cmdutil.Int64PtrIfSet(query.Get("page"))
		if err != nil {
			return nil, err
		}
		pageSize, err := cmdutil.Int64PtrIfSet(query.Get("pageSize"))
		if err != nil {
			return nil, err
		}
		if sectionID != nil {
			return api.ParseResponse[openapi.HomeworksListResponseSchema](client.CommunitySectionHomeworkList(ctx, &openapi.CommunitySectionHomeworkListParams{SectionId: sectionID, IncludeDeleted: includeDeleted, Page: page, PageSize: pageSize}))
		}
		return api.ParseResponse[openapi.SubscribedHomeworksResponseSchema](client.GetSubscribedHomeworks(ctx, &openapi.GetSubscribedHomeworksParams{Page: page, PageSize: pageSize}))
	}, path, url.Values{}, "data", 50, "id")
}
