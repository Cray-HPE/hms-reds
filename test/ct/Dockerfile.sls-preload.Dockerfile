FROM artifactory.algol60.net/docker.io/curlimages/curl

COPY smoke /src/app
ENV PATH="/src/app:${PATH}"

USER root
RUN chown  -R 65534:65534 /src
USER 65534:65534

# this is inherited from the hms-test container
CMD [ "/bin/sh", "-c", "sleep 5 && curl http://cray-sls:8376/v1/hardware -d @/src/app/sls_payload.json" ]