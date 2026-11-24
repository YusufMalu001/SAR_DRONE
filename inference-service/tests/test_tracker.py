"""Unit tests for ByteTracker identity continuity on synthetic sequences."""

import numpy as np
import pytest

from app.models.base import RawDetection
from app.pipeline.tracker import ByteTracker, STrack, TrackState


def test_tracker_identity_continuity_synthetic_sequence():
    """Verifies that an object moving across consecutive frames maintains one persistent track_id."""
    tracker = ByteTracker(high_thresh=0.5, low_thresh=0.2, match_thresh=0.3, max_age=10)
    tracker.reset()

    # Synthetic moving target: starts at x=0.10, moves 0.02 each frame
    initial_track_id = None
    frames_tracked = 0

    for f in range(20):
        x = 0.10 + f * 0.02
        y = 0.20 + f * 0.01
        w, h = 0.08, 0.12
        conf = 0.85

        raw_det = RawDetection(class_id=0, confidence=conf, x=x, y=y, w=w, h=h)
        active_tracks = tracker.update([raw_det], class_names=["person"])

        if active_tracks:
            assert len(active_tracks) == 1
            track = active_tracks[0]
            if initial_track_id is None:
                initial_track_id = track.track_id
            else:
                assert track.track_id == initial_track_id, (
                    f"Frame {f}: Track ID changed from {initial_track_id} to {track.track_id}"
                )
            assert track.class_name == "person"
            assert track.state == TrackState.CONFIRMED
            frames_tracked += 1

    # Confirmed track was maintained across all update frames
    assert frames_tracked >= 19
    assert initial_track_id is not None


def test_tracker_second_stage_low_score_recovery():
    """Verifies ByteTrack recovers an existing track when detection confidence drops into low-score pool."""
    tracker = ByteTracker(high_thresh=0.6, low_thresh=0.2, match_thresh=0.3)
    tracker.reset()

    # Frame 0: High score detection
    det0 = RawDetection(class_id=0, confidence=0.88, x=0.3, y=0.3, w=0.1, h=0.1)
    tracks0 = tracker.update([det0], class_names=["person"])
    assert len(tracks0) == 1
    t_id = tracks0[0].track_id

    # Frame 1: Low score detection (e.g. occlusion/blur with conf=0.35, below high_thresh)
    det1 = RawDetection(class_id=0, confidence=0.35, x=0.31, y=0.31, w=0.1, h=0.1)
    tracks1 = tracker.update([det1], class_names=["person"])

    # ByteTrack stage 2 should match to low-score detection and retain persistent ID
    assert len(tracks1) == 1
    assert tracks1[0].track_id == t_id
    assert abs(tracks1[0].score - 0.35) < 1e-4
