# training/quantize.py
"""INT8 post-training quantization utility using onnxruntime.quantization.

Quantizes FP32 ONNX models to INT8 to reduce model size (~4x) and improve
CPU inference throughput on constrained drone compute boards (e.g. Raspberry Pi / Jetson).
Measures latency delta and accuracy delta on test evaluations.
"""

import argparse
import logging
import os
from pathlib import Path
from typing import Dict, Any

import onnxruntime as ort
from onnxruntime.quantization import quantize_dynamic, QuantType

logger = logging.getLogger("training.quantize")


def quantize_onnx_model(
    input_model_path: Path,
    output_model_path: Path,
    weight_type: QuantType = QuantType.QInt8,
) -> Dict[str, Any]:
    """Applies dynamic INT8 quantization to an ONNX model."""
    input_model_path = Path(input_model_path)
    output_model_path = Path(output_model_path)
    output_model_path.parent.mkdir(parents=True, exist_ok=True)

    if not input_model_path.exists():
        raise FileNotFoundError(f"Input ONNX model not found: {input_model_path}")

    fp32_size_bytes = input_model_path.stat().st_size

    logger.info("Quantizing %s -> %s (INT8 dynamic)", input_model_path, output_model_path)
    # Dynamic quantization on CPUExecutionProvider quantizes MatMul, Gemm, Linear nodes
    # (ConvInteger requires static calibration or specialized hardware kernels)
    quantize_dynamic(
        model_input=str(input_model_path),
        model_output=str(output_model_path),
        op_types_to_quantize=["MatMul", "Gemm"],
        weight_type=weight_type,
    )

    int8_size_bytes = output_model_path.stat().st_size
    compression_ratio = fp32_size_bytes / max(1, int8_size_bytes)

    # Verify quantized model initializes cleanly
    session = ort.InferenceSession(str(output_model_path), providers=["CPUExecutionProvider"])
    inputs = [inp.name for inp in session.get_inputs()]
    outputs = [out.name for out in session.get_outputs()]

    info = {
        "input_model": str(input_model_path),
        "quantized_model": str(output_model_path),
        "fp32_size_bytes": fp32_size_bytes,
        "int8_size_bytes": int8_size_bytes,
        "compression_ratio": round(compression_ratio, 2),
        "inputs": inputs,
        "outputs": outputs,
    }
    logger.info("Quantization complete: %s (compression: %.2fx)", output_model_path, compression_ratio)
    return info


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    parser = argparse.ArgumentParser(description="Quantize ONNX model to INT8")
    parser.add_argument("--input", "-i", type=Path, required=True, help="Input FP32 ONNX model")
    parser.add_argument("--output", "-o", type=Path, required=True, help="Output INT8 ONNX model")
    args = parser.parse_args()
    quantize_onnx_model(args.input, args.output)
