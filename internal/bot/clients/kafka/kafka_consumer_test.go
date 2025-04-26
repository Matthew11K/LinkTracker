package kafka_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"syscall"

	"sync"
	"testing"
	"time"

	kafkaClient "github.com/central-university-dev/go-Matthew11K/internal/bot/clients/kafka"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	segkafka "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
)

type MockMessageHandler struct {
	updates []*models.LinkUpdate
	mu      sync.Mutex
}

func (m *MockMessageHandler) HandleUpdate(_ context.Context, update *models.LinkUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	slog.Info("MockMessageHandler: получено обновление", slog.Int64("id", update.ID), slog.String("url", update.URL))
	m.updates = append(m.updates, update)

	return nil
}

func prepareTopicConfigs(topics []string) []segkafka.TopicConfig {
	logger := slog.Default().With(slog.String("component", "prepareTopicConfigs"))
	topicConfigs := make([]segkafka.TopicConfig, 0, len(topics))

	for _, topic := range topics {
		topicConfigs = append(topicConfigs, segkafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		})

		logger.Info("Подготовлена конфигурация для топика", slog.String("topic", topic))
	}

	return topicConfigs
}

func handleCreateTopicsError(err error, attempt int) {
	logger := slog.Default().With(slog.String("component", "handleCreateTopicsError"))
	logger.Warn("Ошибка при вызове CreateTopics", slog.Int("attempt", attempt), slog.Any("error", err),
		slog.Duration("retry_after", 5*time.Second))

	var netErr net.Error

	switch {
	case errors.As(err, &netErr) && netErr.Timeout():
		logger.Warn("Таймаут операции CreateTopics")
	case errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNREFUSED):
		logger.Warn("Сетевая ошибка при вызове CreateTopics", slog.Any("error_details", err))
	case errors.Is(err, segkafka.TopicAuthorizationFailed):
		logger.Warn("Topic Authorization Failed")
	case errors.Is(err, segkafka.GroupCoordinatorNotAvailable) || errors.Is(err, segkafka.NotController):
		logger.Warn("Брокер еще не полностью готов", slog.Any("kafka_error", err))
	case errors.Is(err, segkafka.RequestTimedOut):
		logger.Warn("Таймаут запроса на стороне Kafka")
	default:
		logger.Warn("Получена неизвестная или другая ошибка Kafka", slog.Any("error_details", err))
	}
}

func processTopicErrors(resp *segkafka.CreateTopicsResponse, attempt int) (bool, error) {
	logger := slog.Default().With(slog.String("component", "processTopicErrors"))
	allCreatedOrExists := true

	var lastErr error

	if resp == nil || resp.Errors == nil {
		logger.Error("Получен пустой ответ или пустой список ошибок от CreateTopics, хотя ошибки запроса не было", slog.Int("attempt", attempt))
		return false, fmt.Errorf("пустой ответ от CreateTopics")
	}

	for topicName, topicErrCode := range resp.Errors {
		switch {
		case topicErrCode != nil && !errors.Is(topicErrCode, segkafka.TopicAlreadyExists):
			specificErr := fmt.Errorf("ошибка создания топика %s: %w", topicName, topicErrCode)
			logger.Warn(specificErr.Error())
			lastErr = specificErr
			allCreatedOrExists = false
		case topicErrCode == nil:
			logger.Info("Топик успешно создан", slog.String("topic", topicName))
		default:
			logger.Info("Топик уже существует", slog.String("topic", topicName))
		}
	}

	return allCreatedOrExists, lastErr
}

func createTopicsAdmin(ctx context.Context, brokers []string, topics ...string) error {
	logger := slog.Default().With(slog.String("component", "createTopicsAdmin"))
	topicConfigs := prepareTopicConfigs(topics)

	transport := &segkafka.Transport{
		DialTimeout: 10 * time.Second,
		IdleTimeout: 30 * time.Second,
	}
	defer transport.CloseIdleConnections()

	client := &segkafka.Client{
		Addr:      segkafka.TCP(brokers...),
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	deadline := time.Now().Add(90 * time.Second)

	var lastErr error

	attempt := 1

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			logger.Error("Контекст отменен во время ожидания создания топиков", slog.Any("error", ctx.Err()))
			return fmt.Errorf("контекст отменен во время создания топиков: %w", ctx.Err())
		default:
		}

		logger.Info("Попытка создания топиков через AdminClient", slog.Int("attempt", attempt), slog.Any("topics", topics))

		createCtx, createCancel := context.WithTimeout(ctx, 25*time.Second)
		resp, err := client.CreateTopics(createCtx, &segkafka.CreateTopicsRequest{
			Topics:       topicConfigs,
			ValidateOnly: false,
		})

		createCancel()

		if err != nil {
			lastErr = err
			handleCreateTopicsError(err, attempt)
			time.Sleep(5 * time.Second)

			attempt++

			continue
		}

		allCreatedOrExists, err := processTopicErrors(resp, attempt)
		if err != nil {
			lastErr = err
		}

		if allCreatedOrExists {
			logger.Info("Все запрошенные топики успешно созданы или уже существовали.")
			return nil
		}

		logger.Warn("Не все топики созданы/существуют, попытка через 5 секунд", slog.Int("attempt", attempt))
		time.Sleep(5 * time.Second)

		attempt++
	}

	logger.Error("Не удалось создать топики после нескольких попыток", slog.Any("last_error", lastErr),
		slog.Duration("total_wait", 90*time.Second))

	finalError := fmt.Errorf("ошибка создания топиков %v через AdminClient после %d попыток", topics, attempt-1)
	if lastErr != nil {
		finalError = fmt.Errorf("%w: %w", finalError, lastErr)
	}

	return finalError
}

func TestKafkaConsumerMock(t *testing.T) {
	messageHandler := &MockMessageHandler{
		updates: make([]*models.LinkUpdate, 0),
	}

	updateTime := time.Now().UTC().Truncate(time.Millisecond)
	testUpdate := &models.LinkUpdate{
		ID:          123,
		URL:         "https://github.com/testuser/testrepo",
		Description: "Test description",
		TgChatIDs:   []int64{456, 789},
		UpdateInfo: &models.UpdateInfo{
			Title:       "Test Title",
			Author:      "Test Author",
			UpdatedAt:   updateTime,
			ContentType: "repository",
			TextPreview: "This is a test preview",
			FullText:    "This is the full text of the update",
		},
	}

	updateForHandler := &models.LinkUpdate{
		ID:          testUpdate.ID,
		URL:         testUpdate.URL,
		Description: testUpdate.Description,
		TgChatIDs:   testUpdate.TgChatIDs,
		UpdateInfo:  testUpdate.UpdateInfo,
	}
	err := messageHandler.HandleUpdate(context.Background(), updateForHandler)
	require.NoError(t, err)

	messageHandler.mu.Lock()
	require.Len(t, messageHandler.updates, 1)
	assert.Equal(t, testUpdate.ID, messageHandler.updates[0].ID)
	assert.Equal(t, testUpdate.URL, messageHandler.updates[0].URL)
	messageHandler.mu.Unlock()
}

func TestKafkaIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем интеграционный тест в режиме short")
	}

	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	ctx := context.Background()

	kafkaContainer, err := tckafka.Run(ctx, "confluentinc/confluent-local:7.5.0")
	require.NoError(t, err, "Не удалось запустить контейнер Kafka")

	defer func() {
		termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer termCancel()

		if err := kafkaContainer.Terminate(termCtx); err != nil {
			logger.Error("Ошибка при остановке контейнера Kafka", slog.Any("error", err))
		}
	}()

	kafkaBrokers, err := kafkaContainer.Brokers(ctx)
	require.NoError(t, err, "Не удалось получить адрес брокеров Kafka")
	require.NotEmpty(t, kafkaBrokers, "Список брокеров Kafka не должен быть пустым")
	logger.Info("Используем брокеры Kafka", slog.Any("brokers", kafkaBrokers))

	topicLinkUpdates := fmt.Sprintf("test-link-updates-%d", time.Now().UnixNano())
	topicDeadLetterQueue := fmt.Sprintf("test-dlq-%d", time.Now().UnixNano())
	logger.Info("Используемые топики", slog.String("updates", topicLinkUpdates), slog.String("dlq", topicDeadLetterQueue))

	logger.Info("Начало явного создания топиков через AdminClient...")

	createCtx, createCancel := context.WithTimeout(ctx, 95*time.Second)
	defer createCancel()

	err = createTopicsAdmin(createCtx, kafkaBrokers, topicLinkUpdates, topicDeadLetterQueue)
	require.NoError(t, err, "Не удалось создать топики через AdminClient")
	logger.Info("Топики успешно созданы или уже существовали.")

	writer := &segkafka.Writer{
		Addr:         segkafka.TCP(kafkaBrokers...),
		Topic:        topicLinkUpdates,
		Balancer:     &segkafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  10 * time.Second,
		RequiredAcks: segkafka.RequireOne,
		Async:        false,
	}
	defer func() {
		logger.Info("Закрытие Kafka writer...")

		err := writer.Close()
		if err != nil {
			logger.Error("Ошибка при закрытии Kafka writer", slog.Any("error", err))
		}

		logger.Info("Kafka writer закрыт.")
	}()

	messageHandler := &MockMessageHandler{
		updates: make([]*models.LinkUpdate, 0),
	}

	consumerGroupID := "test-group-" + fmt.Sprintf("%d", time.Now().UnixNano())
	logger.Info("Создание консьюмера", slog.String("group_id", consumerGroupID))
	consumer := kafkaClient.NewConsumer(
		kafkaBrokers,
		consumerGroupID,
		topicLinkUpdates,
		topicDeadLetterQueue,
		messageHandler,
		logger,
	)

	defer func() {
		logger.Info("Закрытие Kafka consumer...")

		err := consumer.Close()
		if err != nil {
			logger.Error("Ошибка при закрытии Kafka consumer", slog.Any("error", err))
		}

		logger.Info("Kafka consumer закрыт.")
	}()

	consumerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	consumerWG := &sync.WaitGroup{}
	consumerWG.Add(1)

	go func() {
		defer consumerWG.Done()
		logger.Info("Запуск consumer.Start...")

		defer func() {
			if r := recover(); r != nil {
				logger.Error("Паника в consumer.Start", slog.Any("panic", r))
			}
		}()
		consumer.Start(consumerCtx)
		logger.Info("consumer.Start завершился.")
	}()

	logger.Info("Ожидание запуска и стабилизации консьюмера (5 секунд)...")
	time.Sleep(5 * time.Second)

	updateTime := time.Now().UTC().Truncate(time.Millisecond)
	update := &models.LinkUpdate{
		ID:          910,
		URL:         "https://example.com/final-attempt",
		Description: "Final attempt test description",
		TgChatIDs:   []int64{505, 606},
		UpdateInfo: &models.UpdateInfo{
			Title:       "Final Attempt Title",
			Author:      "Tester",
			UpdatedAt:   updateTime,
			ContentType: "blog",
			TextPreview: "Preview text final",
			FullText:    "Full text content final",
		},
	}

	linkUpdate := kafkaClient.LinkUpdateMessage{
		ID:          update.ID,
		URL:         update.URL,
		Description: update.Description,
		TgChatIDs:   update.TgChatIDs,
		UpdateInfo:  update.UpdateInfo,
	}
	jsonData, err := json.Marshal(linkUpdate)
	require.NoError(t, err)

	logger.Info("Отправка сообщения в Kafka...")

	sendCtx, sendCancel := context.WithTimeout(ctx, 20*time.Second)
	defer sendCancel()

	err = writer.WriteMessages(sendCtx, segkafka.Message{
		Key:   []byte(fmt.Sprintf("test-key-%d", update.ID)),
		Value: jsonData,
		Time:  time.Now(),
	})
	require.NoError(t, err, "Ошибка при отправке сообщения в Kafka")
	logger.Info("Сообщение успешно отправлено.")

	receiveDeadline := time.Now().Add(20 * time.Second)

	var receivedUpdate *models.LinkUpdate

	found := false

	for time.Now().Before(receiveDeadline) {
		messageHandler.mu.Lock()
		for _, upd := range messageHandler.updates {
			if upd != nil && upd.ID == update.ID {
				receivedUpdate = upd
				found = true

				break
			}
		}
		messageHandler.mu.Unlock()

		if found {
			logger.Info("Сообщение получено обработчиком.")
			break
		}

		select {
		case <-consumerCtx.Done():
			logger.Error("Контекст консьюмера отменен во время ожидания сообщения")
			t.Fatalf("Контекст консьюмера отменен во время ожидания сообщения: %v", consumerCtx.Err())
		default:
		}

		logger.Debug("Ожидание получения сообщения обработчиком...")
		time.Sleep(500 * time.Millisecond)
	}

	require.True(t, found, "Сообщение ID=%d не было получено обработчиком в течение отведенного времени", update.ID)
	require.NotNil(t, receivedUpdate)

	assert.Equal(t, update.ID, receivedUpdate.ID)
	assert.Equal(t, update.URL, receivedUpdate.URL)
	assert.Equal(t, update.Description, receivedUpdate.Description)
	assert.ElementsMatch(t, update.TgChatIDs, receivedUpdate.TgChatIDs)
	require.NotNil(t, receivedUpdate.UpdateInfo, "UpdateInfo не должно быть nil после получения")
	assert.Equal(t, update.UpdateInfo.Title, receivedUpdate.UpdateInfo.Title)
	assert.Equal(t, update.UpdateInfo.Author, receivedUpdate.UpdateInfo.Author)
	assert.Equal(t, update.UpdateInfo.UpdatedAt, receivedUpdate.UpdateInfo.UpdatedAt.UTC().Truncate(time.Millisecond))
	assert.Equal(t, update.UpdateInfo.ContentType, receivedUpdate.UpdateInfo.ContentType)
	assert.Equal(t, update.UpdateInfo.TextPreview, receivedUpdate.UpdateInfo.TextPreview)
	assert.Equal(t, update.UpdateInfo.FullText, receivedUpdate.UpdateInfo.FullText)

	logger.Info("Остановка консьюмера...")
	cancel()

	logger.Info("Ожидание завершения consumerWG.Wait()...")

	waitCh := make(chan struct{})

	go func() {
		consumerWG.Wait()
		close(waitCh)
	}()
	select {
	case <-waitCh:
		logger.Info("Горутина консьюмера завершилась.")
	case <-time.After(10 * time.Second):
		logger.Error("Таймаут ожидания завершения горутины консьюмера")
		t.Fatal("Таймаут ожидания завершения горутины консьюмера")
	}

	logger.Info("Тест TestKafkaIntegration завершен.")
}
