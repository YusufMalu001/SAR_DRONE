import numpy as np
from simulator.thermal import ThermalRenderingPipeline, ThermalVideoSource


def test_thermal_pipeline_rendering():
    pipeline = ThermalRenderingPipeline(method="radiometric_colormap")
    rgb = np.zeros((100, 100, 3), dtype=np.uint8)
    rgb[30:70, 30:70] = (0, 0, 255)  # Red hotspot

    thermal = pipeline.render(rgb)
    assert thermal.shape == (100, 100, 3)
    assert thermal.dtype == np.uint8
    # Hotspot area should have distinct color response
    assert not np.array_equal(thermal[0, 0], thermal[50, 50])


def test_thermal_video_source_fallback():
    source = ThermalVideoSource(thermal_video_path="nonexistent.mp4", use_pipeline=True)
    rgb = np.ones((60, 80, 3), dtype=np.uint8) * 128
    frame = source.get_frame(rgb)
    assert frame.shape == (60, 80, 3)
    source.release()
