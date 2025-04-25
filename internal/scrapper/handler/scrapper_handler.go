package handler

import (
	"context"
	"errors"
	"math"
	"net/url"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1_scrapper"
	domainerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
)

type ScrapperService interface {
	RegisterChat(ctx context.Context, chatID int64) error

	DeleteChat(ctx context.Context, chatID int64) error

	AddLink(ctx context.Context, chatID int64, url string, tags []string, filters []string) (*models.Link, error)

	RemoveLink(ctx context.Context, chatID int64, url string) (*models.Link, error)

	GetLinks(ctx context.Context, chatID int64) ([]*models.Link, error)

	UpdateNotificationSettings(ctx context.Context, chatID int64, mode models.NotificationMode, digestTime time.Time) error
}

type TagService interface {
	AddTagToLink(ctx context.Context, chatID int64, url string, tag string) error
	RemoveTagFromLink(ctx context.Context, chatID int64, url string, tag string) error
	GetLinksByTag(ctx context.Context, chatID int64, tag string) ([]*models.Link, error)
	GetAllTags(ctx context.Context, chatID int64) ([]string, error)
}

type ScrapperHandler struct {
	scrapperService ScrapperService
	tagService      TagService
}

func NewScrapperHandler(scrapperService ScrapperService, tagService TagService) *ScrapperHandler {
	return &ScrapperHandler{
		scrapperService: scrapperService,
		tagService:      tagService,
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
		var chatNotFoundErr *domainerrors.ErrChatNotFound
		if errors.As(err, &chatNotFoundErr) {
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
	if !req.Link.IsSet() {
		errResp := &v1_scrapper.ApiErrorResponse{
			Description: v1_scrapper.NewOptString("URL не указан"),
		}

		return errResp, &domainerrors.ErrMissingRequiredField{FieldName: "Link"}
	}

	linkURL := req.Link.Value.String()

	link, err := h.scrapperService.AddLink(ctx, params.TgChatID, linkURL, req.Tags, req.Filters)
	if err != nil {
		var unsupportedLinkErr *domainerrors.ErrUnsupportedLinkType
		if errors.As(err, &unsupportedLinkErr) {
			errResp := &v1_scrapper.ApiErrorResponse{
				Description: v1_scrapper.NewOptString("Неподдерживаемый тип ссылки"),
			}

			return errResp, err
		}

		var linkExistsErr *domainerrors.ErrLinkAlreadyExists
		if errors.As(err, &linkExistsErr) {
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
	if !req.Link.IsSet() {
		errResp := &v1_scrapper.LinksDeleteBadRequest{
			Description: v1_scrapper.NewOptString("URL не указан"),
		}

		return errResp, &domainerrors.ErrMissingRequiredField{FieldName: "Link"}
	}

	linkURL := req.Link.Value.String()

	link, err := h.scrapperService.RemoveLink(ctx, params.TgChatID, linkURL)
	if err != nil {
		var linkNotFoundErr *domainerrors.ErrLinkNotFound
		if errors.As(err, &linkNotFoundErr) {
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

func (h *ScrapperHandler) NotificationSettingsPost(ctx context.Context, req *v1_scrapper.UpdateNotificationSettingsRequest,
	params v1_scrapper.NotificationSettingsPostParams) (v1_scrapper.NotificationSettingsPostRes, error) {
	if !req.Mode.IsSet() {
		errResp := &v1_scrapper.NotificationSettingsPostBadRequest{
			Description: v1_scrapper.NewOptString("Режим уведомлений не указан"),
		}

		return errResp, &domainerrors.ErrMissingRequiredField{FieldName: "Mode"}
	}

	var mode models.NotificationMode

	switch req.Mode.Value {
	case v1_scrapper.UpdateNotificationSettingsRequestModeInstant:
		mode = models.NotificationModeInstant
	case v1_scrapper.UpdateNotificationSettingsRequestModeDigest:
		mode = models.NotificationModeDigest
	default:
		errResp := &v1_scrapper.NotificationSettingsPostBadRequest{
			Description: v1_scrapper.NewOptString("Неизвестный режим уведомлений"),
		}

		return errResp, &domainerrors.ErrInvalidValue{FieldName: "Mode", Value: string(req.Mode.Value)}
	}

	var digestTime time.Time

	if mode == models.NotificationModeDigest {
		if !req.DigestHour.IsSet() || !req.DigestMinute.IsSet() {
			errResp := &v1_scrapper.NotificationSettingsPostBadRequest{
				Description: v1_scrapper.NewOptString("Для режима дайджеста необходимо указать время доставки"),
			}

			return errResp, &domainerrors.ErrMissingRequiredField{FieldName: "DigestTime"}
		}

		now := time.Now().UTC()
		digestTime = time.Date(now.Year(), now.Month(), now.Day(),
			int(req.DigestHour.Value), int(req.DigestMinute.Value), 0, 0, time.UTC)
	}

	err := h.scrapperService.UpdateNotificationSettings(ctx, params.TgChatID, mode, digestTime)
	if err != nil {
		var chatNotFoundErr *domainerrors.ErrChatNotFound
		if errors.As(err, &chatNotFoundErr) {
			errResp := &v1_scrapper.NotificationSettingsPostNotFound{
				Description: v1_scrapper.NewOptString("Чат не найден"),
			}

			return errResp, err
		}

		errResp := &v1_scrapper.NotificationSettingsPostBadRequest{
			Description: v1_scrapper.NewOptString("Ошибка при обновлении настроек уведомлений"),
		}

		return errResp, err
	}

	return &v1_scrapper.NotificationSettingsPostOK{}, nil
}
