package handlers

import (
	"context"
	"errors"
	"math"
	"net/url"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1/v1_scrapper"
	domainerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
)

type ScrapperService interface {
	RegisterChat(ctx context.Context, chatID int64) error

	DeleteChat(ctx context.Context, chatID int64) error

	AddLink(ctx context.Context, chatID int64, url string, tags []string, filters []string) (*models.Link, error)

	RemoveLink(ctx context.Context, chatID int64, url string) (*models.Link, error)

	GetLinks(ctx context.Context, chatID int64) ([]*models.Link, error)

	CheckUpdates(ctx context.Context) error
}

type ScrapperHandler struct {
	scrapperService ScrapperService
}

func NewScrapperHandler(scrapperService ScrapperService) *ScrapperHandler {
	return &ScrapperHandler{
		scrapperService: scrapperService,
	}
}

func (h *ScrapperHandler) TgChatIDPost(ctx context.Context, params v1_scrapper.TgChatIDPostParams) (v1_scrapper.TgChatIDPostRes, error) {
	if err := h.scrapperService.RegisterChat(ctx, params.ID); err != nil {
		errResp := &v1_scrapper.ApiErrorResponse{
			Description: v1_scrapper.NewOptString("Ошибка при регистрации чата"),
		}

		return errResp, err
	}

	return &v1_scrapper.TgChatIDPostOK{}, nil
}

func (h *ScrapperHandler) TgChatIDDelete(ctx context.Context,
	params v1_scrapper.TgChatIDDeleteParams) (v1_scrapper.TgChatIDDeleteRes, error) {
	if err := h.scrapperService.DeleteChat(ctx, params.ID); err != nil {
		if _, ok := err.(*domainerrors.ErrChatNotFound); ok {
			errResp := &v1_scrapper.TgChatIDDeleteNotFound{
				Description: v1_scrapper.NewOptString("Чат не найден"),
			}

			return errResp, err
		}

		errResp := &v1_scrapper.TgChatIDDeleteBadRequest{
			Description: v1_scrapper.NewOptString("Ошибка при удалении чата"),
		}

		return errResp, err
	}

	return &v1_scrapper.TgChatIDDeleteOK{}, nil
}

func (h *ScrapperHandler) LinksPost(ctx context.Context, req *v1_scrapper.AddLinkRequest,
	params v1_scrapper.LinksPostParams) (v1_scrapper.LinksPostRes, error) {
	var linkURL string
	if req.Link.IsSet() {
		linkURL = req.Link.Value.String()
	} else {
		errResp := &v1_scrapper.ApiErrorResponse{
			Description: v1_scrapper.NewOptString("URL не указан"),
		}

		return errResp, errors.New("URL не указан")
	}

	link, err := h.scrapperService.AddLink(ctx, params.TgChatID, linkURL, req.Tags, req.Filters)
	if err != nil {
		if _, ok := err.(*domainerrors.ErrUnsupportedLinkType); ok {
			errResp := &v1_scrapper.ApiErrorResponse{
				Description: v1_scrapper.NewOptString("Неподдерживаемый тип ссылки"),
			}

			return errResp, err
		}

		if _, ok := err.(*domainerrors.ErrLinkAlreadyExists); ok {
			errResp := &v1_scrapper.ApiErrorResponse{
				Description: v1_scrapper.NewOptString("Ссылка уже существует"),
			}

			return errResp, err
		}

		errResp := &v1_scrapper.ApiErrorResponse{
			Description: v1_scrapper.NewOptString("Ошибка при добавлении ссылки"),
		}

		return errResp, err
	}

	resp := &v1_scrapper.LinkResponse{
		ID:      v1_scrapper.NewOptInt64(link.ID),
		Tags:    link.Tags,
		Filters: link.Filters,
	}

	if parsedURL, err := url.Parse(link.URL); err == nil {
		resp.URL = v1_scrapper.NewOptURI(*parsedURL)
	}

	return resp, nil
}

func (h *ScrapperHandler) LinksDelete(ctx context.Context, req *v1_scrapper.RemoveLinkRequest,
	params v1_scrapper.LinksDeleteParams) (v1_scrapper.LinksDeleteRes, error) {
	var linkURL string
	if req.Link.IsSet() {
		linkURL = req.Link.Value.String()
	} else {
		errResp := &v1_scrapper.LinksDeleteBadRequest{
			Description: v1_scrapper.NewOptString("URL не указан"),
		}

		return errResp, errors.New("URL не указан")
	}

	link, err := h.scrapperService.RemoveLink(ctx, params.TgChatID, linkURL)
	if err != nil {
		if _, ok := err.(*domainerrors.ErrLinkNotFound); ok {
			errResp := &v1_scrapper.LinksDeleteNotFound{
				Description: v1_scrapper.NewOptString("Ссылка не найдена"),
			}

			return errResp, err
		}

		errResp := &v1_scrapper.LinksDeleteBadRequest{
			Description: v1_scrapper.NewOptString("Ошибка при удалении ссылки"),
		}

		return errResp, err
	}

	resp := &v1_scrapper.LinkResponse{
		ID:      v1_scrapper.NewOptInt64(link.ID),
		Tags:    link.Tags,
		Filters: link.Filters,
	}

	if parsedURL, err := url.Parse(link.URL); err == nil {
		resp.URL = v1_scrapper.NewOptURI(*parsedURL)
	}

	return resp, nil
}

func (h *ScrapperHandler) LinksGet(ctx context.Context, params v1_scrapper.LinksGetParams) (v1_scrapper.LinksGetRes, error) {
	links, err := h.scrapperService.GetLinks(ctx, params.TgChatID)
	if err != nil {
		errResp := &v1_scrapper.ApiErrorResponse{
			Description: v1_scrapper.NewOptString("Ошибка при получении списка ссылок"),
		}

		return errResp, err
	}

	linksCount := len(links)

	var size int32
	if linksCount > math.MaxInt32 {
		size = math.MaxInt32
	} else {
		size = int32(linksCount)
	}

	resp := &v1_scrapper.ListLinksResponse{
		Links: make([]v1_scrapper.LinkResponse, 0, len(links)),
		Size:  v1_scrapper.NewOptInt32(size),
	}

	for _, link := range links {
		linkResp := v1_scrapper.LinkResponse{
			ID:      v1_scrapper.NewOptInt64(link.ID),
			Tags:    link.Tags,
			Filters: link.Filters,
		}

		if parsedURL, err := url.Parse(link.URL); err == nil {
			linkResp.URL = v1_scrapper.NewOptURI(*parsedURL)
		}

		resp.Links = append(resp.Links, linkResp)
	}

	return resp, nil
}
