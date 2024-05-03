from locust import HttpUser, TaskSet, task, between


class UserBehavior(TaskSet):
    @task(1)
    def create_event(self):
        headers = {'Content-Type': 'application/json'}
        payload = {
            "name": "Test Event",
            "date": "2024-05-04T12:00:00Z",
            "totalTickets": 100
        }
        for _ in range(10):
            response = self.client.post("/api/v1/events", json=payload, headers=headers)
            if response.status_code != 200:
                print("Failed to create event:", response.text)


class WebsiteUser(HttpUser):
    tasks = [UserBehavior]
    wait_time = between(1, 3)
    host = "http://localhost:8000"
