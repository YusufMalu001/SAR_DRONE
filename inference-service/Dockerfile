FROM python:3.11-slim-bookworm

WORKDIR /srv

COPY requirements.txt .
RUN pip install --no-cache-dir --extra-index-url https://download.pytorch.org/whl/cpu -r requirements.txt && \
    pip install --no-cache-dir --force-reinstall opencv-python-headless==4.10.0.84

COPY app ./app
COPY training ./training
COPY scripts ./scripts
COPY tests ./tests
COPY train_person_model.py train_thermal_model.py train_hazard_classifier.py ./

EXPOSE 8001

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8001"]
