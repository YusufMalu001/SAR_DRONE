from unittest.mock import MagicMock
from simulator.scenario import Scenario
from simulator.runner import ScenarioRunner


def test_scenario_runner_dry_run():
    scenario = Scenario.load("scenarios/flood.yaml")
    runner = ScenarioRunner(scenario=scenario, redis_client=None, speed=0.0)

    res = runner.run(max_frames=10)
    assert res["scenario"] == "flood"
    assert res["frames_replayed"] == 10
    assert res["telemetry_published"] > 0
    assert "telemetry_digest" in res


def test_scenario_runner_with_mock_redis():
    scenario = Scenario.load("scenarios/flood.yaml")
    mock_redis = MagicMock()

    runner = ScenarioRunner(
        scenario=scenario,
        redis_client=mock_redis,
        speed=0.0,
        detections_stream="mock_detections",
        telemetry_stream="mock_telemetry",
    )

    res = runner.run(max_frames=20)
    assert res["frames_replayed"] == 20
    assert mock_redis.xadd.called

    # Verify calls to mock_detections and mock_telemetry
    streams_called = {call.args[0] for call in mock_redis.xadd.call_args_list}
    assert "mock_telemetry" in streams_called
    assert "mock_detections" in streams_called
