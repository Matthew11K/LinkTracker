package clients

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	v1_scrapper "github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1_scrapper"
	"github.com/central-university-dev/go-Matthew11K/internal/common/httputil"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	domainerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type ScrapperClient struct {
	client *v1_scrapper.Client
}

func NewScrapperClient(baseURL string, cfg *config.Config, logger *slog.Logger) (*ScrapperClient, error) {
	if baseURL == "" {
		baseURL = "http://link_tracker_scrapper:8081"
	}

	resilientClient := httputil.CreateResilientOpenAPIClient(cfg, logger, "scrapper_service")

	client, err := v1_scrapper.NewClient(baseURL, v1_scrapper.WithClient(resilientClient))
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании клиента скраппера: %w", err)
	}

	return &ScrapperClient{
		client: client,
	}, nil
}

func (c *ScrapperClient) RegisterChat(ctx context.Context, chatID int64) error {
	params := v1_scrapper.TgChatIDPostParams{
		ID: chatID,
	}

	resp, err := c.client.TgChatIDPost(ctx, params)
	if err != nil {
		return &domainerrors.ErrInternalServer{Message: "не удалось выполнить запрос: " + err.Error()}
	}

	switch resp.(type) {
	case *v1_scrapper.TgChatIDPostOK:
		return nil
	default:
		return &domainerrors.ErrInternalServer{Message: "неожиданный ответ от сервера"}
	}
}

func (c *ScrapperClient) DeleteChat(ctx context.Context, chatID int64) error {
	params := v1_scrapper.TgChatIDDeleteParams{
		ID: chatID,
	}

	resp, err := c.client.TgChatIDDelete(ctx, params)
	if err != nil {
		return &domainerrors.ErrInternalServer{Message: "не удалось выполнить запрос: " + err.Error()}
	}

	switch resp.(type) {
	case *v1_scrapper.TgChatIDDeleteOK:
		return nil
	case *v1_scrapper.TgChatIDDeleteNotFound:
		return &domainerrors.ErrChatNotFound{ChatID: chatID}
	default:
		return &domainerrors.ErrInternalServer{Message: fmt.Sprintf("неожиданный ответ от сервера: %T", resp)}
	}
}

func (c *ScrapperClient) AddLink(ctx context.Context, chatID int64, linkURL string, tags, filters []string) (*models.Link, error) {
	parsedURL, err := url.Parse(linkURL)
	if err != nil {
		return nil, &domainerrors.ErrInvalidArgument{Message: "некорректный URL"}
	}

	req := &v1_scrapper.AddLinkRequest{
		Link:    v1_scrapper.NewOptURI(*parsedURL),
		Tags:    tags,
		Filters: filters,
	}

	params := v1_scrapper.LinksPostParams{
		TgChatID: chatID,
	}

	resp, err := c.client.LinksPost(ctx, req, params)
	if err != nil {
		if err.Error() == "Ссылка уже существует" {
			return nil, &domainerrors.ErrLinkAlreadyExists{URL: linkURL}
		}

		if err.Error() == "Неподдерживаемый тип ссылки" {
			return nil, &domainerrors.ErrUnsupportedLinkType{URL: linkURL}
		}

		return nil, &domainerrors.ErrInternalServer{Message: "не удалось выполнить запрос: " + err.Error()}
	}

	linkResp, ok := resp.(*v1_scrapper.LinkResponse)
	if !ok {
		return nil, &domainerrors.ErrInternalServer{Message: "неожиданный ответ от сервера"}
	}

	link := &models.Link{
		ID:      linkResp.ID.Or(0),
		Tags:    linkResp.Tags,
		Filters: linkResp.Filters,
	}

	if linkResp.URL.IsSet() {
		link.URL = linkResp.URL.Value.String()
	}

	return link, nil
}

func (c *ScrapperClient) RemoveLink(ctx context.Context, chatID int64, linkURL string) (*models.Link, error) {
	parsedURL, err := url.Parse(linkURL)
	if err != nil {
		return nil, &domainerrors.ErrInvalidArgument{Message: "некорректный URL"}
	}

	req := &v1_scrapper.RemoveLinkRequest{
		Link: v1_scrapper.NewOptURI(*parsedURL),
	}

	params := v1_scrapper.LinksDeleteParams{
		TgChatID: chatID,
	}

	resp, err := c.client.LinksDelete(ctx, req, params)
	if err != nil {
		if err.Error() == "Ссылка не найдена" {
			return nil, &domainerrors.ErrLinkNotFound{URL: linkURL}
		}

		return nil, &domainerrors.ErrInternalServer{Message: "не удалось выполнить запрос: " + err.Error()}
	}

	linkResp, ok := resp.(*v1_scrapper.LinkResponse)
	if !ok {
		return nil, &domainerrors.ErrInternalServer{Message: "неожиданный ответ от сервера"}
	}

	link := &models.Link{
		ID:      linkResp.ID.Or(0),
		Tags:    linkResp.Tags,
		Filters: linkResp.Filters,
	}

	if linkResp.URL.IsSet() {
		link.URL = linkResp.URL.Value.String()
	}

	return link, nil
}

func (c *ScrapperClient) GetLinks(ctx context.Context, chatID int64) ([]*models.Link, error) {
	params := v1_scrapper.LinksGetParams{
		TgChatID: chatID,
	}

	resp, err := c.client.LinksGet(ctx, params)
	if err != nil {
		return nil, &domainerrors.ErrInternalServer{Message: "не удалось выполнить запрос: " + err.Error()}
	}

	listResp, ok := resp.(*v1_scrapper.ListLinksResponse)
	if !ok {
		return nil, &domainerrors.ErrInternalServer{Message: "неожиданный ответ от сервера"}
	}

	links := make([]*models.Link, 0, len(listResp.Links))

	for i := range listResp.Links {
		linkResp := &listResp.Links[i]
		link := &models.Link{
			ID:      linkResp.ID.Or(0),
			Tags:    linkResp.Tags,
			Filters: linkResp.Filters,
		}

		if linkResp.URL.IsSet() {
			link.URL = linkResp.URL.Value.String()
		}

		links = append(links, link)
	}

	return links, nil
}

func (c *ScrapperClient) UpdateNotificationSettings(ctx context.Context, chatID int64, mode models.NotificationMode,
	digestTime time.Time) error {
	var apiMode v1_scrapper.UpdateNotificationSettingsRequestMode

	var req v1_scrapper.UpdateNotificationSettingsRequest

	switch mode {
	case models.NotificationModeInstant:
		apiMode = v1_scrapper.UpdateNotificationSettingsRequestModeInstant
		req.Mode = apiMode
	case models.NotificationModeDigest:
		apiMode = v1_scrapper.UpdateNotificationSettingsRequestModeDigest

		req.Mode = apiMode
		if !digestTime.IsZero() {
			req.DigestHour = v1_scrapper.NewOptInt32(int32(digestTime.Hour()))     //nolint:gosec // значения часов и минут валидны
			req.DigestMinute = v1_scrapper.NewOptInt32(int32(digestTime.Minute())) //nolint:gosec // значения часов и минут валидны
		}
	default:
		return &domainerrors.ErrUnknownNotificationMode{Mode: string(mode)}
	}

	params := v1_scrapper.NotificationSettingsPostParams{
		TgChatID: chatID,
	}

	resp, err := c.client.NotificationSettingsPost(ctx, &req, params)
	if err != nil {
		return fmt.Errorf("не удалось обновить настройки уведомлений: %w", err)
	}

	switch resp.(type) {
	case *v1_scrapper.NotificationSettingsPostOK:
		return nil
	case *v1_scrapper.NotificationSettingsPostNotFound:
		return &domainerrors.ErrChatNotFound{ChatID: chatID}
	default:
		return &domainerrors.ErrInternalServer{Message: "неожиданный ответ сервера при обновлении настроек уведомлений"}
	}
}
