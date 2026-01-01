package render

import "context"

type Renderer interface {
	RenderHome(ctx context.Context, page HomePage) ([]byte, error)
	RenderPost(ctx context.Context, page PostPage) ([]byte, error)
	RenderSeries(ctx context.Context, page SeriesPage) ([]byte, error)
	RenderList(ctx context.Context, page ListPage) ([]byte, error)
	RenderNotFound(ctx context.Context, page NotFoundPage) ([]byte, error)
	RenderArchives(ctx context.Context, page ArchivesPage) ([]byte, error)
	RenderTagsPage(ctx context.Context, page TagsPage) ([]byte, error)
	RenderCategoriesPage(ctx context.Context, page CategoriesPage) ([]byte, error)
}
