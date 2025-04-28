#!/usr/bin/env python3

import os
import time
import requests
import structlog
from urllib.parse import urlparse

# Configure structlog
structlog.configure(
    processors=[
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.JSONRenderer(),
    ],
)
logger = structlog.get_logger()


def update_nextdns(endpoint):
    """Update NextDNS with the current WAN IP by calling the endpoint directly."""
    # Validate endpoint
    parsed_url = urlparse(endpoint)
    if not all([parsed_url.scheme, parsed_url.netloc]):
        logger.error("Invalid NextDNS endpoint", endpoint=endpoint)
        return False

    try:
        response = requests.get(endpoint)
        if response.status_code == 200:
            logger.info("Successfully updated NextDNS", endpoint=endpoint)
            return True
        else:
            logger.error(
                "Failed to update NextDNS",
                status_code=response.status_code,
                response=response.text,
            )
            return False
    except Exception as e:
        logger.error("Error updating NextDNS", error=str(e), endpoint=endpoint)
        return False


def main():
    # Get NextDNS endpoint from environment variable or use default
    nextdns_endpoint = os.environ.get("NEXTDNS_ENDPOINT")

    if not nextdns_endpoint:
        logger.error("NEXTDNS_ENDPOINT environment variable is not set")
        return

    # Get update interval from environment variable or use default (5 minutes)
    interval = int(os.environ.get("UPDATE_INTERVAL_SECONDS", 300))

    logger.info(
        "Starting NextDNS IP updater",
        endpoint=nextdns_endpoint,
        interval_seconds=interval,
    )

    while True:
        success = update_nextdns(nextdns_endpoint)
        logger.info("Update cycle completed", success=success)
        time.sleep(interval)


if __name__ == "__main__":
    main()
