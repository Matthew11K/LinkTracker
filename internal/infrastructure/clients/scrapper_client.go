package clients

import (
	"context"
	"fmt"
	"net/url"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1/v1_scrapper"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type ScrapperClient struct {
	client *v1_scrapper.Client
}

func NewScrapperClient(baseURL string) (*ScrapperClient, error) {
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}

	client, err := v1_scrapper.NewClient(baseURL)
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
		return &errors.ErrInternalServer{Message: "не удалось выполнить запрос: " + err.Error()}
	}

	switch resp.(type) {
	case *v1_scrapper.TgChatIDPostOK:
		return nil
	default:
		return &errors.ErrInternalServer{Message: "неожиданный ответ от сервера"}
	}
}

func (c *ScrapperClient) DeleteChat(ctx context.Context, chatID int64) error {
	params := v1_scrapper.TgChatIDDeleteParams{
		ID: chatID,
	}

	resp, err := c.client.TgChatIDDelete(ctx, params)
	if err != nil {
		return &errors.ErrInternalServer{Message: "не удалось выполнить запрос: " + err.Error()}
	}

	switch resp.(type) {
	case *v1_scrapper.TgChatIDDeleteOK:
		return nil
	case *v1_scrapper.TgChatIDDeleteNotFound:
		return &errors.ErrChatNotFound{ChatID: chatID}
	default:
		return &errors.ErrInternalServer{Message: "неожиданный ответ от сервера"}
	}
}

func (c *ScrapperClient) AddLink(ctx context.Context, chatID int64, linkURL string, tags, filters []string) (*models.Link, error) {
	parsedURL, err := url.Parse(linkURL)
	if err != nil {
		return nil, &errors.ErrInvalidArgument{Message: "некорректный URL"}
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
			return nil, &errors.ErrLinkAlreadyExists{URL: linkURL}
		}

		if err.Error() == "Неподдерживаемый тип ссылки" {
			return nil, &errors.ErrUnsupportedLinkType{URL: linkURL}
		}

		return nil, &errors.ErrInternalServer{Message: "не удалось выполнить запрос: " + err.Error()}
	}

	linkResp, ok := resp.(*v1_scrapper.LinkResponse)
	if !ok {
		return nil, &errors.ErrInternalServer{Message: "неожиданный ответ от сервера"}
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
		return nil, &errors.ErrInvalidArgument{Message: "некорректный URL"}
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
			return nil, &errors.ErrLinkNotFound{URL: linkURL}
		}

		return nil, &errors.ErrInternalServer{Message: "не удалось выполнить запрос: " + err.Error()}
	}

	linkResp, ok := resp.(*v1_scrapper.LinkResponse)
	if !ok {
		return nil, &errors.ErrInternalServer{Message: "неожиданный ответ от сервера"}
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
		return nil, &errors.ErrInternalServer{Message: "не удалось выполнить запрос: " + err.Error()}
	}

	listResp, ok := resp.(*v1_scrapper.ListLinksResponse)
	if !ok {
		return nil, &errors.ErrInternalServer{Message: "неожиданный ответ от сервера"}
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
