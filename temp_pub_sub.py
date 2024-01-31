import grpc
import threading
from concurrent import futures
import time
from typing import Dict, List
import uuid
import pubsub_pb2 as pb
import pubsub_pb2_grpc as pb_grpc

class Broker(pb_grpc.PubSubServiceServicer):
    def __init__(self, port):
        self.port = port
        self.subscribers = {}
        self.topic_subscribers_mutex = {}
        self.lock = threading.RLock()
        self.server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
        pb_grpc.add_PubSubServiceServicer_to_server(self, self.server)

    def start(self):
        self.server.add_insecure_port('[::]:' + str(self.port))
        self.server.start()
        try:
            while True:
                time.sleep(86400)
        except KeyboardInterrupt:
            self.stop()

    def stop(self):
        self.server.stop(0)

    def subscribe(self, request, context):
        with self.lock:
            key = (request.topic, request.subscriber_id)
            if request.topic not in self.subscribers:
                self.subscribers[request.topic] = {}
            self.subscribers[request.topic][request.subscriber_id] = context
            self.topic_subscribers_mutex[key] = threading.Lock()

        while True:
            if context.is_active():
                time.sleep(1)
            else:
                break

    def unsubscribe(self, request, context):
        with self.lock:
            key = (request.topic, request.subscriber_id)
            if request.topic not in self.subscribers or request.subscriber_id not in self.subscribers[request.topic]:
                return pb.UnsubscribeResponse(success=False)
            del self.subscribers[request.topic][request.subscriber_id]
            del self.topic_subscribers_mutex[key]
        return pb.UnsubscribeResponse(success=True)

    def publish(self, request, context):
        with self.lock:
            broker_subscribers = []
            for subscriber_id, subscriber_context in self.subscribers.get(request.topic, {}).items():
                key = (request.topic, subscriber_id)
                self.topic_subscribers_mutex[key].acquire()
                try:
                    subscriber_context.send(pb.Message(topic=request.topic, message=request.message))
                except grpc.RpcError as e:
                    print(f"Error sending message to subscriber {subscriber_id}: {e}")
                    broker_subscribers.append(key)
                finally:
                    self.topic_subscribers_mutex[key].release()

            self.remove_broken_subscribers(broker_subscribers)

        if broker_subscribers:
            return pb.PublishResponse(success=False)
        return pb.PublishResponse(success=True)

    def remove_broken_subscribers(self, keys):
        with self.lock:
            for key in keys:
                del self.subscribers[key[0]][key[1]]
                del self.topic_subscribers_mutex[key]

class Consumer:
    def __init__(self, broker_address):
        self.id = uuid.uuid4().int
        self.broker_address = broker_address
        self.conn = grpc.insecure_channel(broker_address)
        self.client = pb_grpc.PubSubServiceStub(self.conn)
        self.messages = []
        self.subscriptions = {}
        self.lock = threading.RLock()
        self.ctx = threading.Event()

    def subscribe(self, topic):
        with self.lock:
            if topic in self.subscriptions:
                return
            stream = self.client.Subscribe(pb.SubscriberRequest(topic=topic, subscriber_id=self.id))
            self.subscriptions[topic] = (stream, threading.Event())
            threading.Thread(target=self.receive, args=(topic,), daemon=True).start()

    def unsubscribe(self, topic):
        with self.lock:
            if topic not in self.subscriptions:
                return
            self.subscriptions[topic][0].cancel()
            del self.subscriptions[topic]
            self.client.Unsubscribe(pb.UnsubscribeRequest(topic=topic, subscriber_id=self.id))

    def receive(self, topic):
        while not self.subscriptions[topic][1].is_set():
            try:
                msg = next(self.subscriptions[topic][0])
                with self.lock:
                    self.messages.append(msg)
            except grpc.RpcError:
                break

    def close(self):
        with self.lock:
            for topic in list(self.subscriptions.keys()):
                self.unsubscribe(topic)
            self.conn.close()

class Publisher:
    def __init__(self, broker_address):
        self.broker_address = broker_address
        self.conn = grpc.insecure_channel(broker_address)
        self.client = pb_grpc.PubSubServiceStub(self.conn)

    def publish(self, topic, message):
        self.client.Publish(pb.PublishRequest(topic=topic, message=message))

    def close(self):
        self.conn.close()

def run_example():
    broker = Broker(50051)
    threading.Thread(target=broker.start, daemon=True).start()

    consumer = Consumer('localhost:50051')
    publisher = Publisher('localhost:50051')

    consumer.subscribe('example_topic')
    time.sleep(1)  # Give some time for subscription to complete

    publisher.publish('example_topic', b'Hello, World!')

    time.sleep(1)  # Give some time for message to be received
    messages = consumer.messages

    print("Received Messages:", messages)

    consumer.close()
    publisher.close()
    broker.stop()

if __name__ == '__main__':
    run_example()

