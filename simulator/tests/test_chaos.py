from unittest.mock import MagicMock
from simulator.scenario import ConnectivityEvent
from simulator.chaos import ChaosScheduler, ConnectivityChaosHook


def test_chaos_scheduler_timeline():
    events = [
        ConnectivityEvent(
            type="link_down",
            t_start_s=2.0,
            t_end_s=5.0,
            action="set_offline",
            description="test link down",
        )
    ]

    mock_hook = MagicMock(spec=ConnectivityChaosHook)
    scheduler = ChaosScheduler(events=events, hook=mock_hook)

    # Before event
    logs1 = scheduler.update(1.0)
    assert len(logs1) == 0
    mock_hook.cut_link.assert_not_called()

    # During event
    logs2 = scheduler.update(2.5)
    assert len(logs2) == 1
    mock_hook.cut_link.assert_called_once_with("set_offline")

    # Still during event: should not re-trigger
    logs3 = scheduler.update(3.5)
    assert len(logs3) == 0

    # After event
    logs4 = scheduler.update(5.5)
    assert len(logs4) == 1
    mock_hook.restore_link.assert_called_once_with("set_offline")

    # Reset
    scheduler.reset()
    assert mock_hook.restore_link.call_count == 2
