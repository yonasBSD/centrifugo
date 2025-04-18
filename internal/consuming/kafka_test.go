//go:build integration

package consuming

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/centrifugal/centrifugo/v6/internal/api"
	"github.com/centrifugal/centrifugo/v6/internal/configtypes"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	testKafkaBrokerURL = "localhost:29092"
)

// MockDispatcher implements the Dispatcher interface for testing.
type MockDispatcher struct {
	onDispatchCommand     func(ctx context.Context, method string, data []byte) error
	onDispatchPublication func(ctx context.Context, channels []string, pub api.ConsumedPublication) error
}

func (m *MockDispatcher) DispatchCommand(ctx context.Context, method string, data []byte) error {
	return m.onDispatchCommand(ctx, method, data)
}

func (m *MockDispatcher) DispatchPublication(ctx context.Context, channels []string, pub api.ConsumedPublication) error {
	return m.onDispatchPublication(ctx, channels, pub)
}

func produceTestMessage(topic string, message []byte, headers []kgo.RecordHeader) error {
	// Create a new client
	client, err := kgo.NewClient(kgo.SeedBrokers(testKafkaBrokerURL))
	if err != nil {
		return fmt.Errorf("failed to create Kafka client: %w", err)
	}
	defer client.Close()

	// Produce a message
	err = client.ProduceSync(context.Background(), &kgo.Record{
		Topic:     topic,
		Partition: 0,
		Value:     message,
		Headers:   headers,
	}).FirstErr()
	if err != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}
	return nil
}

func produceManyRecords(records ...*kgo.Record) error {
	client, err := kgo.NewClient(kgo.SeedBrokers(testKafkaBrokerURL))
	if err != nil {
		return fmt.Errorf("failed to create Kafka client: %w", err)
	}
	defer client.Close()
	err = client.ProduceSync(context.Background(), records...).FirstErr()
	if err != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}
	return nil
}

func produceTestMessageToPartition(topic string, message []byte, partition int32) error {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(testKafkaBrokerURL),
		kgo.RecordPartitioner(kgo.ManualPartitioner()),
	)
	if err != nil {
		return fmt.Errorf("failed to create Kafka client: %w", err)
	}
	defer client.Close()

	res := client.ProduceSync(context.Background(), &kgo.Record{
		Topic: topic, Partition: partition, Value: message})
	if res.FirstErr() != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}
	producedPartition := res[0].Record.Partition
	if producedPartition != partition {
		return fmt.Errorf("failed to produce message to partition %d, produced to %d", partition, producedPartition)
	}
	return nil
}

func createTestTopic(ctx context.Context, topicName string, numPartitions int32, replicationFactor int16) error {
	cl, err := kgo.NewClient(kgo.SeedBrokers(testKafkaBrokerURL))
	if err != nil {
		return fmt.Errorf("failed to create Kafka client: %w", err)
	}
	defer cl.Close()

	client := kadm.NewClient(cl)
	defer client.Close()

	// Create the topic
	resp, err := client.CreateTopics(ctx, numPartitions, replicationFactor, nil, topicName)
	if err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	for _, topic := range resp.Sorted() {
		if topic.Err != nil {
			if strings.Contains(topic.Err.Error(), "TOPIC_ALREADY_EXISTS") {
				continue
			}
			return fmt.Errorf("failed to create topic '%s': %v", topic.Topic, topic.Err)
		}
	}
	return nil
}

func waitCh(t *testing.T, ch chan struct{}, timeout time.Duration, failureMessage string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		require.Fail(t, failureMessage)
	}
}

func waitAnyCh(t *testing.T, channels []chan struct{}, timeout time.Duration, failureMessage string) {
	t.Helper()

	anyCh := make(chan struct{}, 1)

	for _, ch := range channels {
		go func(c chan struct{}) {
			select {
			case <-c:
				select {
				case anyCh <- struct{}{}:
				default:
				}
			case <-time.After(timeout):
				// let main handle timeout
			}
		}(ch)
	}

	select {
	case <-anyCh:
		// One of the channels received a signal
	case <-time.After(timeout):
		require.Fail(t, failureMessage)
	}
}

func TestKafkaConsumer_GreenScenario(t *testing.T) {
	t.Parallel()
	testKafkaTopic := "centrifugo_consumer_test_" + uuid.New().String()
	testMethod := "method"
	testPayload := []byte(`{"key":"value"}`)

	testEvent := api.MethodWithRequestPayload{
		Method:  testMethod,
		Payload: api.JSONRawOrString(testPayload),
	}
	testMessage, _ := json.Marshal(testEvent)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := createTestTopic(ctx, testKafkaTopic, 1, 1)
	require.NoError(t, err)

	config := KafkaConfig{
		Brokers:       []string{testKafkaBrokerURL}, // Adjust as needed
		Topics:        []string{testKafkaTopic},
		ConsumerGroup: uuid.New().String(),
	}

	eventReceived := make(chan struct{})
	consumerClosed := make(chan struct{})

	consumer, err := NewKafkaConsumer(config, &MockDispatcher{
		onDispatchCommand: func(ctx context.Context, method string, data []byte) error {
			require.Equal(t, "", method)
			require.Equal(t, testMessage, data)
			close(eventReceived)
			return nil
		},
	}, testCommon(prometheus.NewRegistry()))
	require.NoError(t, err)

	go func() {
		err := consumer.Run(ctx)
		require.ErrorIs(t, err, context.Canceled)
		close(consumerClosed)
	}()

	err = produceTestMessage(testKafkaTopic, testMessage, nil)
	require.NoError(t, err)

	waitCh(t, eventReceived, 30*time.Second, "timeout waiting for event")
	cancel()
	waitCh(t, consumerClosed, 30*time.Second, "timeout waiting for consumer closed")
}

func TestKafkaConsumer_SeveralConsumers(t *testing.T) {
	t.Parallel()
	testKafkaTopic := "centrifugo_consumer_test_" + uuid.New().String()
	testMethod := "method"
	testPayload := []byte(`{"key":"value"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := createTestTopic(ctx, testKafkaTopic, 1, 1)
	require.NoError(t, err)

	config := KafkaConfig{
		Brokers:       []string{testKafkaBrokerURL}, // Adjust as needed
		Topics:        []string{testKafkaTopic},
		ConsumerGroup: uuid.New().String(),
	}

	eventReceived := make(chan struct{})
	consumerClosed := make(chan struct{})

	testEvent := api.MethodWithRequestPayload{
		Method:  testMethod,
		Payload: api.JSONRawOrString(testPayload),
	}
	testMessage, _ := json.Marshal(testEvent)

	for i := 0; i < 3; i++ {
		consumer, err := NewKafkaConsumer(config, &MockDispatcher{
			onDispatchCommand: func(ctx context.Context, method string, data []byte) error {
				require.Equal(t, testMessage, data)
				close(eventReceived)
				return nil
			},
		}, testCommon(prometheus.NewRegistry()))
		require.NoError(t, err)

		go func() {
			err := consumer.Run(ctx)
			require.ErrorIs(t, err, context.Canceled)
			consumerClosed <- struct{}{}
		}()
	}

	err = produceTestMessage(testKafkaTopic, testMessage, nil)
	require.NoError(t, err)

	waitCh(t, eventReceived, 30*time.Second, "timeout waiting for event")
	cancel()
	waitCh(t, consumerClosed, 30*time.Second, "timeout waiting for consumer closed")
	waitCh(t, consumerClosed, 30*time.Second, "timeout waiting for consumer closed")
	waitCh(t, consumerClosed, 30*time.Second, "timeout waiting for consumer closed")
}

func TestKafkaConsumer_RetryAfterDispatchError(t *testing.T) {
	t.Parallel()
	testKafkaTopic := "centrifugo_consumer_test_" + uuid.New().String()
	testMethod := "method"
	testPayload := []byte(`{"key":"value"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := createTestTopic(ctx, testKafkaTopic, 1, 1)
	require.NoError(t, err)

	config := KafkaConfig{
		Brokers:       []string{testKafkaBrokerURL},
		Topics:        []string{testKafkaTopic},
		ConsumerGroup: uuid.New().String(),
	}

	successCh := make(chan struct{})
	consumerClosed := make(chan struct{})

	retryCount := 0
	numFailures := 3

	mockDispatcher := &MockDispatcher{
		onDispatchCommand: func(ctx context.Context, method string, data []byte) error {
			if retryCount < numFailures {
				retryCount++
				return errors.New("dispatch error")
			}
			close(successCh)
			return nil
		},
	}
	consumer, err := NewKafkaConsumer(
		config,
		mockDispatcher, testCommon(prometheus.NewRegistry()))
	require.NoError(t, err)

	go func() {
		err := consumer.Run(ctx)
		require.ErrorIs(t, err, context.Canceled)
		close(consumerClosed)
	}()

	testEvent := api.MethodWithRequestPayload{
		Method:  testMethod,
		Payload: api.JSONRawOrString(testPayload),
	}
	testMessage, _ := json.Marshal(testEvent)
	err = produceTestMessage(testKafkaTopic, testMessage, nil)
	require.NoError(t, err)

	waitCh(t, successCh, 30*time.Second, "timeout waiting for successful event process")
	require.Equal(t, numFailures, retryCount)
	cancel()
	waitCh(t, consumerClosed, 30*time.Second, "timeout waiting for consumer closed")
}

// In this test we simulate a scenario where a partition is blocked and the partition consumer
// is stuck on it. We want to make sure that the consumer is not blocked and can still process
// messages from other topics.
func TestKafkaConsumer_BlockedPartitionDoesNotBlockAnotherTopic(t *testing.T) {
	t.Parallel()
	testKafkaTopic1 := "consumer_test_1_" + uuid.New().String()
	testKafkaTopic2 := "consumer_test_1_" + uuid.New().String()

	testPayload1 := []byte(`{"key":"value1"}`)
	testPayload2 := []byte(`{"key":"value2"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := createTestTopic(ctx, testKafkaTopic1, 1, 1)
	require.NoError(t, err)

	err = createTestTopic(ctx, testKafkaTopic2, 1, 1)
	require.NoError(t, err)

	event1Received := make(chan struct{})
	event2Received := make(chan struct{})
	consumerClosed := make(chan struct{})
	doneCh := make(chan struct{})

	config := KafkaConfig{
		Brokers:       []string{testKafkaBrokerURL},
		Topics:        []string{testKafkaTopic1, testKafkaTopic2},
		ConsumerGroup: uuid.New().String(),
	}

	numCalls := 0

	mockDispatcher := &MockDispatcher{
		onDispatchCommand: func(ctx context.Context, method string, data []byte) error {
			if numCalls == 0 {
				numCalls++
				close(event1Received)
				// Block till the event2 received. This must not block the consumer and event2
				// must still be processed successfully.
				<-event2Received
				return nil
			}
			close(event2Received)
			return nil
		},
	}
	consumer, err := NewKafkaConsumer(
		config, mockDispatcher, testCommon(prometheus.NewRegistry()))
	require.NoError(t, err)

	go func() {
		err = produceTestMessage(testKafkaTopic1, testPayload1, nil)
		require.NoError(t, err)

		// Wait until the first message is received to make sure messages read by separate PollRecords calls.
		<-event1Received
		err = produceTestMessage(testKafkaTopic2, testPayload2, nil)
		require.NoError(t, err)
	}()

	go func() {
		err := consumer.Run(ctx)
		require.ErrorIs(t, err, context.Canceled)
		close(consumerClosed)
	}()

	waitCh(t, event2Received, 30*time.Second, "timeout waiting for event 2")
	cancel()
	waitCh(t, consumerClosed, 30*time.Second, "timeout waiting for consumer closed")
	close(doneCh)
}

// In this test we simulate a scenario where a partition is blocked and the partition consumer
// is stuck on it. We want to make sure that the consumer is not blocked and can still process
// messages from other topic partitions.
func TestKafkaConsumer_BlockedPartitionDoesNotBlockAnotherPartition(t *testing.T) {
	partitionBufferSizes := []int{0, 1}

	for _, partitionBufferSize := range partitionBufferSizes {
		t.Run(fmt.Sprintf("partition_buffer_size_%d", partitionBufferSize), func(t *testing.T) {
			t.Parallel()
			testKafkaTopic1 := "consumer_test_1_" + uuid.New().String()

			testPayload1 := []byte(`{"key":"value1"}`)
			testPayload2 := []byte(`{"key":"value2"}`)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := createTestTopic(ctx, testKafkaTopic1, 2, 1)
			require.NoError(t, err)

			event1Received := make(chan struct{})
			event2Received := make(chan struct{})
			consumerClosed := make(chan struct{})
			doneCh := make(chan struct{})

			config := KafkaConfig{
				Brokers:             []string{testKafkaBrokerURL},
				Topics:              []string{testKafkaTopic1},
				ConsumerGroup:       uuid.New().String(),
				PartitionBufferSize: partitionBufferSize,
			}

			numCalls := 0

			mockDispatcher := &MockDispatcher{
				onDispatchCommand: func(ctx context.Context, method string, data []byte) error {
					if numCalls == 0 {
						numCalls++
						close(event1Received)
						// Block till the event2 received. This must not block the consumer and event2
						// must still be processed successfully.
						<-event2Received
						return nil
					}
					close(event2Received)
					return nil
				},
			}
			consumer, err := NewKafkaConsumer(
				config, mockDispatcher, testCommon(prometheus.NewRegistry()))
			require.NoError(t, err)

			go func() {
				err = produceTestMessageToPartition(testKafkaTopic1, testPayload1, 0)
				require.NoError(t, err)

				// Wait until the first message is received to make sure messages read by separate PollRecords calls.
				<-event1Received
				err = produceTestMessageToPartition(testKafkaTopic1, testPayload2, 1)
				require.NoError(t, err)
			}()

			go func() {
				err := consumer.Run(ctx)
				require.ErrorIs(t, err, context.Canceled)
				close(consumerClosed)
			}()

			waitCh(t, event2Received, 30*time.Second, "timeout waiting for event 2")
			cancel()
			waitCh(t, consumerClosed, 30*time.Second, "timeout waiting for consumer closed")
			close(doneCh)
		})
	}
}
func TestKafkaConsumer_PausePartitions(t *testing.T) {
	t.Parallel()
	testKafkaTopic := "consumer_test_" + uuid.New().String()
	testPayload1 := []byte(`{"key":"value1"}`)
	testPayload2 := []byte(`{"key":"value2"}`)
	testPayload3 := []byte(`{"key":"value3"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := createTestTopic(ctx, testKafkaTopic, 1, 1)
	require.NoError(t, err)

	event1Received := make(chan struct{})
	event2Received := make(chan struct{})
	event3Received := make(chan struct{})
	consumerClosed := make(chan struct{})
	doneCh := make(chan struct{})

	fetchSubmittedCh := make(chan kgo.FetchTopicPartition)
	beforePauseCh := make(chan topicPartition)

	config := KafkaConfig{
		Brokers:       []string{testKafkaBrokerURL},
		Topics:        []string{testKafkaTopic},
		ConsumerGroup: uuid.New().String(),

		PartitionBufferSize: 1,
	}

	numCalls := 0

	unblockCh := make(chan struct{})

	testConfig := testOnlyConfig{
		fetchTopicPartitionSubmittedCh: fetchSubmittedCh,
		topicPartitionBeforePauseCh:    beforePauseCh,
	}

	mockDispatcher := &MockDispatcher{
		onDispatchCommand: func(ctx context.Context, method string, data []byte) error {
			numCalls++
			if numCalls == 1 {
				close(event1Received)
				<-unblockCh
				return nil
			} else if numCalls == 2 {
				close(event2Received)
				return nil
			}
			close(event3Received)
			return nil
		},
	}
	consumer, err := NewKafkaConsumer(
		config, mockDispatcher, testCommon(prometheus.NewRegistry()))
	require.NoError(t, err)

	consumer.testOnlyConfig = testConfig

	go func() {
		err = produceTestMessage(testKafkaTopic, testPayload1, nil)
		require.NoError(t, err)
		<-fetchSubmittedCh
		<-event1Received

		err = produceTestMessage(testKafkaTopic, testPayload2, nil)
		require.NoError(t, err)
		<-fetchSubmittedCh

		// At this point message 1 is being processed and the next produced message must
		// cause a partition pause.
		err = produceTestMessage(testKafkaTopic, testPayload3, nil)
		require.NoError(t, err)
		<-beforePauseCh // Wait for triggering the partition pause.

		// Unblock the message processing.
		close(unblockCh)
		<-fetchSubmittedCh
	}()

	go func() {
		err := consumer.Run(ctx)
		require.ErrorIs(t, err, context.Canceled)
		close(consumerClosed)
	}()

	waitCh(t, event1Received, 30*time.Second, "timeout waiting for event 1")
	waitCh(t, event2Received, 30*time.Second, "timeout waiting for event 2")
	waitCh(t, event3Received, 30*time.Second, "timeout waiting for event 3")
	cancel()
	waitCh(t, consumerClosed, 30*time.Second, "timeout waiting for consumer closed")
	close(doneCh)
}

func TestKafkaConsumer_WorksCorrectlyInLoadedTopic(t *testing.T) {
	t.Skip()
	t.Parallel()

	testCases := []struct {
		numPartitions   int32
		numMessages     int
		partitionBuffer int
	}{
		//{numPartitions: 1, numMessages: 1000, partitionBuffer: 1}
		{numPartitions: 10, numMessages: 10000, partitionBuffer: 1},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("partitions=%d,messages=%d,buffer=%d", tc.numPartitions, tc.numMessages, tc.partitionBuffer)
		t.Run(name, func(t *testing.T) {
			testKafkaTopic := "consumer_test_" + uuid.New().String()

			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
			defer cancel()

			err := createTestTopic(ctx, testKafkaTopic, tc.numPartitions, 1)
			require.NoError(t, err)

			consumerClosed := make(chan struct{})
			doneCh := make(chan struct{})

			numMessages := tc.numMessages
			messageCh := make(chan struct{}, numMessages)

			mockDispatcher := &MockDispatcher{
				onDispatchCommand: func(ctx context.Context, method string, data []byte) error {
					// Emulate delay due to some work.
					time.Sleep(20 * time.Millisecond)
					messageCh <- struct{}{}
					return nil
				},
			}
			config := KafkaConfig{
				Brokers:             []string{testKafkaBrokerURL},
				Topics:              []string{testKafkaTopic},
				ConsumerGroup:       uuid.New().String(),
				PartitionBufferSize: tc.partitionBuffer,
			}
			consumer, err := NewKafkaConsumer(
				config, mockDispatcher, testCommon(prometheus.NewRegistry()))
			require.NoError(t, err)

			var records []*kgo.Record
			for i := 0; i < numMessages; i++ {
				records = append(records, &kgo.Record{Topic: testKafkaTopic, Value: []byte(`{"hello": "` + strconv.Itoa(i) + `"}`)})
				if (i+1)%100 == 0 {
					err = produceManyRecords(records...)
					if err != nil {
						t.Fatal(err)
					}
					records = nil
					t.Logf("produced %d messages", i+1)
				}
			}

			t.Logf("all messages produced, 3, 2, 1, go!")
			time.Sleep(time.Second)

			go func() {
				err := consumer.Run(ctx)
				require.ErrorIs(t, err, context.Canceled)
				close(consumerClosed)
			}()

			var numProcessed int64
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Second):
						t.Logf("processed %d messages", atomic.LoadInt64(&numProcessed))
					}
				}
			}()

			for i := 0; i < numMessages; i++ {
				<-messageCh
				atomic.AddInt64(&numProcessed, 1)
			}
			t.Logf("all messages processed")
			cancel()
			waitCh(t, consumerClosed, 30*time.Second, "timeout waiting for consumer closed")
			close(doneCh)
		})
	}
}

// TestKafkaConsumer_TestPauseAfterResumeRace tests a scenario where a partition was
// paused after it was resumed and partition never processed any messages after that.
func TestKafkaConsumer_TestPauseAfterResumeRace(t *testing.T) {
	t.Parallel()
	testKafkaTopic := "consumer_test_" + uuid.New().String()
	testPayload1 := []byte(`{"input":"value1"}`)
	testPayload2 := []byte(`{"input":"value2"}`)
	testPayload3 := []byte(`{"input":"value3"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	err := createTestTopic(ctx, testKafkaTopic, 1, 1)
	require.NoError(t, err)

	consumerClosed := make(chan struct{})
	doneCh := make(chan struct{})

	messageCh := make(chan struct{}, 128)

	partitionBeforePauseCh := make(chan topicPartition)
	partitionPauseProceedCh := make(chan struct{})
	fetchSubmittedCh := make(chan kgo.FetchTopicPartition)

	config := KafkaConfig{
		Brokers:             []string{testKafkaBrokerURL},
		Topics:              []string{testKafkaTopic},
		ConsumerGroup:       uuid.New().String(),
		PartitionBufferSize: 1,
	}

	count := 0
	proceedCh := make(chan struct{})
	firstMessageReceived := make(chan struct{})

	mockDispatcher := &MockDispatcher{
		onDispatchCommand: func(ctx context.Context, method string, data []byte) error {
			if count == 0 {
				close(firstMessageReceived)
				// Block until we are allowed to proceed
				t.Logf("waiting for proceed")
				<-proceedCh
				t.Logf("proceeding")
			}
			count++
			messageCh <- struct{}{}
			t.Logf("message processed")
			return nil
		},
	}
	consumer, err := NewKafkaConsumer(
		config, mockDispatcher, testCommon(prometheus.NewRegistry()))
	require.NoError(t, err)

	consumer.testOnlyConfig = testOnlyConfig{
		topicPartitionBeforePauseCh:    partitionBeforePauseCh,
		topicPartitionPauseProceedCh:   partitionPauseProceedCh,
		fetchTopicPartitionSubmittedCh: fetchSubmittedCh,
	}

	go func() {
		err := consumer.Run(ctx)
		require.ErrorIs(t, err, context.Canceled)
		close(consumerClosed)
	}()

	go func() {
		err = produceTestMessage(testKafkaTopic, testPayload1, nil)
		require.NoError(t, err)

		<-fetchSubmittedCh
		t.Logf("fetch 1 submitted")

		// This one should be buffered.
		err = produceTestMessage(testKafkaTopic, testPayload2, nil)
		require.NoError(t, err)

		<-fetchSubmittedCh
		t.Logf("fetch 2 submitted")

		// This message pauses the partition consumer.
		err = produceTestMessage(testKafkaTopic, testPayload3, nil)
		require.NoError(t, err)
		<-partitionBeforePauseCh
		close(proceedCh)
		// Give consumer some time to process messages, so we can be sure that resume was called.
		time.Sleep(time.Second)
		// And now we can proceed so that partition will be paused after resume.
		close(partitionPauseProceedCh)
		// Wait for the third message submitted for processing.
		<-fetchSubmittedCh
	}()

	for i := 0; i < 3; i++ {
		<-messageCh
	}

	cancel()
	waitCh(t, consumerClosed, 30*time.Second, "timeout waiting for consumer closed")
	close(doneCh)
}

func TestKafkaConsumer_GreenScenario_PublicationDataMode(t *testing.T) {
	t.Parallel()
	testKafkaTopic := "centrifugo_consumer_test_" + uuid.New().String()
	testChannels := []string{"channel1", "channel2"}
	testPayload := []byte(`{"key":"value"}`)
	testIdempotencyKey := "test-idempotency-key"
	const testDelta = true

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := createTestTopic(ctx, testKafkaTopic, 1, 1)
	require.NoError(t, err)

	config := KafkaConfig{
		Brokers:       []string{testKafkaBrokerURL}, // Adjust as needed
		Topics:        []string{testKafkaTopic},
		ConsumerGroup: uuid.New().String(),
		PublicationDataMode: configtypes.KafkaPublicationDataModeConfig{
			Enabled:              true,
			ChannelsHeader:       "centrifugo-channels",
			IdempotencyKeyHeader: "centrifugo-idempotency-key",
			DeltaHeader:          "centrifugo-delta",
		},
	}

	event1Received := make(chan struct{})
	event2Received := make(chan struct{})
	consumerClosed := make(chan struct{})

	count := 0

	mockDispatcher := &MockDispatcher{
		onDispatchPublication: func(
			ctx context.Context, channels []string, pub api.ConsumedPublication,
		) error {
			require.Nil(t, pub.Tags)
			if count == 0 {
				require.Len(t, channels, 1)
				require.Equal(t, testChannels[0], channels[0])
				require.Equal(t, testPayload, pub.Data)
				require.Equal(t, testIdempotencyKey, pub.IdempotencyKey)
				require.Equal(t, testDelta, pub.Delta)
				close(event1Received)
			} else {
				require.Equal(t, testChannels, channels)
				require.Equal(t, testPayload, pub.Data)
				require.Equal(t, testIdempotencyKey, pub.IdempotencyKey)
				require.Equal(t, testDelta, pub.Delta)
				close(event2Received)
			}
			count++
			return nil
		},
	}

	consumer, err := NewKafkaConsumer(
		config, mockDispatcher, testCommon(prometheus.NewRegistry()))
	require.NoError(t, err)

	go func() {
		err := consumer.Run(ctx)
		require.ErrorIs(t, err, context.Canceled)
		close(consumerClosed)
	}()

	err = produceTestMessage(testKafkaTopic, testPayload, []kgo.RecordHeader{
		{
			Key:   config.PublicationDataMode.ChannelsHeader,
			Value: []byte(testChannels[0]),
		},
		{
			Key:   config.PublicationDataMode.IdempotencyKeyHeader,
			Value: []byte(testIdempotencyKey),
		},
		{
			Key:   config.PublicationDataMode.DeltaHeader,
			Value: []byte(fmt.Sprintf("%v", testDelta)),
		},
	})
	require.NoError(t, err)

	err = produceTestMessage(testKafkaTopic, testPayload, []kgo.RecordHeader{
		{
			Key:   config.PublicationDataMode.ChannelsHeader,
			Value: []byte(strings.Join(testChannels, ",")),
		},
		{
			Key:   config.PublicationDataMode.IdempotencyKeyHeader,
			Value: []byte(testIdempotencyKey),
		},
		{
			Key:   config.PublicationDataMode.DeltaHeader,
			Value: []byte(fmt.Sprintf("%v", testDelta)),
		},
	})
	require.NoError(t, err)

	waitCh(t, event1Received, 30*time.Second, "timeout waiting for event1")
	waitCh(t, event2Received, 30*time.Second, "timeout waiting for event2")
	cancel()
	waitCh(t, consumerClosed, 30*time.Second, "timeout waiting for consumer closed")
}
