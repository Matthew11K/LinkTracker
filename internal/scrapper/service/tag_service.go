package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository"
)

type TagService struct {
	linkRepo repository.LinkRepository
	chatRepo repository.ChatRepository
	logger   *slog.Logger
}

func NewTagService(linkRepo repository.LinkRepository, chatRepo repository.ChatRepository, logger *slog.Logger) *TagService {
	return &TagService{
		linkRepo: linkRepo,
		chatRepo: chatRepo,
		logger:   logger,
	}
}

func (s *TagService) AddTagToLink(ctx context.Context, chatID int64, url, tag string) error {
	_, err := s.chatRepo.FindByID(ctx, chatID)
	if err != nil {
		return err
	}

	link, err := s.linkRepo.FindByURL(ctx, url)
	if err != nil {
		return err
	}

	exists, err := s.chatRepo.ExistsChatLink(ctx, chatID, link.ID)
	if err != nil {
		return err
	}

	if !exists {
		return &errors.ErrLinkNotFound{URL: url}
	}

	for _, existingTag := range link.Tags {
		if existingTag == tag {
			return &errors.ErrTagAlreadyExists{Tag: tag, URL: url}
		}
	}

	link.Tags = append(link.Tags, tag)

	if err := s.linkRepo.SaveTags(ctx, link.ID, link.Tags); err != nil {
		return fmt.Errorf("ошибка при сохранении тегов: %w", err)
	}

	s.logger.Info("Тег добавлен к ссылке",
		"tag", tag,
		"url", url,
		"chatID", chatID,
	)

	return nil
}

func (s *TagService) RemoveTagFromLink(ctx context.Context, chatID int64, url, tag string) error {
	_, err := s.chatRepo.FindByID(ctx, chatID)
	if err != nil {
		return err
	}

	link, err := s.linkRepo.FindByURL(ctx, url)
	if err != nil {
		return err
	}

	exists, err := s.chatRepo.ExistsChatLink(ctx, chatID, link.ID)
	if err != nil {
		return err
	}

	if !exists {
		return &errors.ErrLinkNotFound{URL: url}
	}

	tagFound := false
	newTags := make([]string, 0, len(link.Tags))

	for _, existingTag := range link.Tags {
		if existingTag == tag {
			tagFound = true
			continue
		}

		newTags = append(newTags, existingTag)
	}

	if !tagFound {
		return &errors.ErrTagNotFound{Tag: tag, URL: url}
	}

	link.Tags = newTags

	if err := s.linkRepo.SaveTags(ctx, link.ID, link.Tags); err != nil {
		return fmt.Errorf("ошибка при сохранении тегов: %w", err)
	}

	s.logger.Info("Тег удален из ссылки",
		"tag", tag,
		"url", url,
		"chatID", chatID,
	)

	return nil
}

func (s *TagService) GetLinksByTag(ctx context.Context, chatID int64, tag string) ([]*models.Link, error) {
	_, err := s.chatRepo.FindByID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	links, err := s.linkRepo.FindByChatID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	result := make([]*models.Link, 0)

	for _, link := range links {
		for _, linkTag := range link.Tags {
			if linkTag == tag {
				result = append(result, link)
				break
			}
		}
	}

	return result, nil
}

func (s *TagService) GetAllTags(ctx context.Context, chatID int64) ([]string, error) {
	_, err := s.chatRepo.FindByID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	links, err := s.linkRepo.FindByChatID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	tagMap := make(map[string]struct{})

	for _, link := range links {
		for _, tag := range link.Tags {
			tagMap[tag] = struct{}{}
		}
	}

	result := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		result = append(result, tag)
	}

	return result, nil
}
