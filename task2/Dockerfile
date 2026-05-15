FROM rabbitmq:4.0-management

ENV PLUGIN_URL=https://github.com/noxdafox/rabbitmq-message-deduplication/releases/download/0.7.3/plugins-rmqv4.0.x-erl26.2-elx1.16.zip

RUN apt-get update && apt-get install -y curl unzip && \
    curl -fL -o /tmp/plugin.zip $PLUGIN_URL && \
    unzip /tmp/plugin.zip -d /plugins && \
    rm /tmp/plugin.zip && \
    apt-get remove -y curl unzip && \
    apt-get clean

RUN rabbitmq-plugins enable --offline rabbitmq_message_deduplication