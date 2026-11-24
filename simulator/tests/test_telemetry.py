from simulator.scenario import Scenario
from simulator.telemetry import TelemetrySynthesizer, compute_telemetry_digest


def test_byte_identical_telemetry_reproducibility():
    """Confirms that re-running trajectory synthesis with the same seed yields byte-identical output."""
    scenario = Scenario.load("scenarios/flood.yaml")
    synth = TelemetrySynthesizer(scenario)

    run_a = synth.generate(duration_s=20.0)
    digest_a = compute_telemetry_digest(run_a)

    run_b = synth.generate(duration_s=20.0)
    digest_b = compute_telemetry_digest(run_b)

    assert digest_a == digest_b
    assert len(run_a) == len(run_b)

    # Check sample-by-sample byte equality
    for s_a, s_b in zip(run_a, run_b):
        assert s_a.lat == s_b.lat
        assert s_a.lon == s_b.lon
        assert s_a.alt == s_b.alt
        assert s_a.heading == s_b.heading
        assert s_a.pitch == s_b.pitch
        assert s_a.roll == s_b.roll
        assert s_a.is_gps_valid == s_b.is_gps_valid


def test_different_seeds_yield_different_digests():
    scenario_a = Scenario.load("scenarios/flood.yaml")
    scenario_b = Scenario.load("scenarios/flood.yaml")
    scenario_b.flight.seed = 9999

    synth_a = TelemetrySynthesizer(scenario_a)
    synth_b = TelemetrySynthesizer(scenario_b)

    digest_a = compute_telemetry_digest(synth_a.generate(duration_s=10.0))
    digest_b = compute_telemetry_digest(synth_b.generate(duration_s=10.0))

    assert digest_a != digest_b


def test_telemetry_payload_schema_keys():
    scenario = Scenario.load("scenarios/flood.yaml")
    synth = TelemetrySynthesizer(scenario)
    samples = synth.generate(duration_s=1.0)
    assert len(samples) > 0

    payload = samples[0].to_stream_dict("2026-11-24T12:00:00Z")
    assert set(payload.keys()) == {"lat", "lon", "alt", "heading", "timestamp"}
    assert -90.0 <= payload["lat"] <= 90.0
    assert -180.0 <= payload["lon"] <= 180.0
    assert 0.0 <= payload["heading"] <= 360.0
