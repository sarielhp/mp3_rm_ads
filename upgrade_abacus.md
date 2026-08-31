# AI & GPU Upgrade Guide for `abacus`

This document compiles the complete hardware audit, upgrade paths, AI performance benchmarks, architectural designs, and purchasing strategies for upgrading **`abacus`** into a high-performance local AI and audio processing workstation.

---

## 1. System Baseline Audit: `abacus`

Hardware audit performed on `abacus` via SSH:

| Component | Specification | Notes |
|---|---|---|
| **System Model** | HP Pavilion Desktop TP01-2xxx | Compact Mini-Tower Chassis |
| **Motherboard** | HP 8906 (Erica6) | AMD B550/A520 equivalent, AM4 socket |
| **CPU** | AMD Ryzen 7 5700G (8 Cores / 16 Threads) | Zen 3, 3.8 GHz Base / 4.6 GHz Boost, 65W TDP |
| **Current GPU** | Integrated AMD Radeon Vega Graphics | PCIe x16 slot is currently **empty and available** |
| **RAM** | 32 GB DDR4-3200 (2 × 16 GB UDIMMs) | Dual-channel configured |
| **Power Supply** | Factory OEM HP PSU (180W / 310W) | 12V-only proprietary form factor (no 8-pin PCIe cable) |
| **Chassis Clearance** | ~220 mm max GPU length (2-slot width) | Compact mini-tower layout |
| **Idle Power** | **~15W – 22W from the wall** | Monolithic Cezanne silicon (ultra-low standby power) |

---

## 2. Upgrade Pathways for AI & Whisper

### Path A: Zero-Upgrade Internal Plug & Play (Stock HP PSU)
*Best for zero-hassle Whisper acceleration on a tight budget.*

* **GPU**: **NVIDIA GeForce RTX 3050 6GB** (~$160 new)
* **Power Requirement**: 70W directly from PCIe slot (**No extra power cables needed**).
* **Work Needed**: Plug directly into the open PCIe x16 slot.
* **AI Speed**: Transcribes 1-hour audio in **~2.5 – 3 minutes** (~20x real-time).
* **Total Investment**: **~$160**

---

### Path B: High-Performance Internal GPU (HP 500W PSU Swap)
*Best balance of speed, 12GB–16GB VRAM, and clean internal aesthetics (0 desk clutter).*

* **PSU Upgrade**: **HP 500W Gold OEM Power Supply (Part # `L77487-001`)** (~$45–$55 on eBay). Drops directly into the TP01 chassis and provides 2× 8-pin PCIe cables.
* **GPU Options**:
  1. **NVIDIA RTX 3060 12GB (Compact 2-fan, e.g. EVGA XC / ZOTAC Twin Edge at ~200mm)**:
     - **Cost**: ~$200–$220 used.
     - **VRAM**: 12 GB GDDR6.
     - **Speed**: Transcribes 1-hour audio in **~90 seconds** (~40x real-time). Runs 8B LLMs (Llama 3, Qwen) with 32k context.
     - **Total Cost**: **~$250 – $270**
  2. **NVIDIA RTX 4060 Ti 16GB (Compact 2-fan, e.g. MSI Ventus 2X at 199mm)**:
     - **Cost**: ~$390–$430.
     - **VRAM**: **16 GB GDDR6** (165W TDP).
     - **Speed**: Transcribes 1-hour audio in **~60 seconds** (~60x real-time). Runs 8B–14B LLMs and large contexts.
     - **Total Cost**: **~$440 – $480**

---

### Path C: The Ultimate AI Flagship — External NVIDIA RTX 3090 24GB
*Best for serious machine learning, training, LoRA fine-tuning, and running 70B models.*

By mounting the GPU in an external dock, all chassis length, cooling, and power limits are removed.

```
┌──────────────────────────────────────────────┐
│                   `abacus`                   │
│   [Motherboard PCIe x16 Slot]                │
└──────────────────────┬───────────────────────┘
                       │ (High-Speed Shielded PCIe 4.0 Riser Cable)
                       ▼
┌──────────────────────────────────────────────┐
│           EXTERNAL AI DOCK / STAND           │
│                                              │
│   [Standard 750W/850W ATX Power Supply]      │
│                     │ (Dedicated 8-pin PCIe) │
│                     ▼                        │
│   [NVIDIA RTX 3090 24GB GDDR6X]              │
└──────────────────────────────────────────────┘
```

* **Components & Cost Breakdown**:
  * Used **NVIDIA RTX 3090 24GB**: **~$650** (eBay / r/hardwareswap)
  * **PCIe x16 Riser Dock / Stand** (e.g. Minisforum DEG1 / ADT-Link): **~$45 – $90**
  * **750W 80+ Gold ATX Power Supply** (MSI / Corsair / Thermaltake): **~$65**
  * **Total Investment**: **~$760 – $805**
* **VRAM**: **24 GB GDDR6X** with **936 GB/s memory bandwidth**.
* **Speed**:
  * Whisper 1-hour audio: **~25 – 35 seconds** (~120x real-time).
  * 70B LLMs (4-bit): **~18 – 22 tokens/sec**.
  * 8B LLMs: **80 – 120+ tokens/sec**.
  * Multi-model stacking: Whisper Large-V3 + Qwen 14B + Vector Embeddings simultaneously in VRAM.
  * Full LoRA / QLoRA model fine-tuning with PyTorch / Unsloth.

---

## 3. Comprehensive Hardware Comparison Table

| Metric | Stock `abacus` (CPU) | RTX 3050 6GB (Internal) | RTX 3060 12GB (Internal) | RTX 4060 Ti 16GB (Internal) | RTX 3090 24GB (External) |
|---|---|---|---|---|---|
| **Mount Location** | N/A | Inside case | Inside case | Inside case | External Dock |
| **VRAM** | Shared DDR4 | 6 GB GDDR6 | 12 GB GDDR6 | 16 GB GDDR6 | **24 GB GDDR6X** |
| **Memory Bandwidth** | 51.2 GB/s | 168 GB/s | 360 GB/s | 288 GB/s | **936 GB/s** |
| **Power Draw (Peak)** | 65W | 70W | 170W | 165W | 350W |
| **Idle System Power** | ~18W | ~22W | ~26W | ~24W | **~28W (P8 Sleep)** |
| **1h Audio Transcribe** | ~11 min | ~2.5 min | ~1.5 min | ~1.0 min | **~25 – 35 sec** |
| **LLM Inference Scope** | Tiny (3B) | Small (7B/8B Q4) | 8B Full / 14B Q4 | 8B Full / 14B / 32B Q4 | **70B Q4 / 32B / 14B Full** |
| **Model Fine-Tuning** | ❌ No | ❌ No | ⚠️ Small LoRA | ⚠️ Medium LoRA | ✅ **Full QLoRA / PyTorch** |
| **Extra Parts Needed** | None | None | HP 500W PSU ($45) | HP 500W PSU ($50) | Dock ($45) + ATX PSU ($65) |
| **Total Cost** | **$0** | **~$160** | **~$250** | **~$450** | **~$780** |

---

## 4. Power Management & Standby Efficiency

### Why `abacus` is Uniquely Power-Efficient:
The Ryzen 7 5700G is a monolithic APU engineered with aggressive C-states. While running, it uses a fraction of the electricity of older Intel K-series or enterprise Xeon workstations:
* **24/7 Idle Cost**: `abacus` + RTX 3090 in deep P8 idle (~28W total) costs **~$35/year** in electricity.
* **Dual-PC / Workstation alternative**: 80W–120W idle costs **~$110–$160/year**.

### Power Control Methods for the External GPU:
1. **Dynamic Runtime Power Management (P8 State — Recommended)**:
   - When no AI job is running, the GPU automatically downclocks to P8 state (~8W draw, fans 100% off).
   - Wakes up in <100ms when `abs` or Ollama runs.
2. **Physical Shutoff via Linux PCIe Rescan (0 Watts)**:
   ```bash
   # 1. Safely disconnect GPU before flipping power switch off:
   echo 1 | sudo tee /sys/bus/pci/devices/0000:01:00.0/remove

   # 2. Re-detect GPU after flipping power switch on:
   echo 1 | sudo tee /sys/bus/pci/rescan
   ```
3. **Power-Sync Docks (e.g. Minisforum DEG1)**:
   - The dock reads the PCIe power-good signal: automatically powers on the external PSU when `abacus` boots/wakes and cuts power when `abacus` sleeps/shuts down.

---

## 5. System RAM Upgrade Analysis: 32 GB vs. 64 GB

* **Motherboard Capacity**: HP 8906 has 2 DDR4 slots supporting up to **64 GB (2 × 32 GB DDR4-3200 UDIMMs)**.
* **Upgrade Cost**: ~$95 – $115 on Amazon/Newegg.
* **Is it worth it for AI?**
  * **For 90% of AI tasks: NO.** AI compute speed is governed by **GPU VRAM bandwidth (936 GB/s)**. System RAM (51.2 GB/s) is 18x slower.
  * Models running in GPU VRAM barely touch system RAM.
  * **Keep the 32 GB RAM** and invest the funds into the GPU.

---

## 6. Comparison: `abacus` + RTX 3090 vs. Apple Silicon

| Feature | `abacus` + NVIDIA RTX 3090 24GB | Apple Mac mini M4 Pro (64GB) | Apple Mac Studio (M5 Ultra 512GB) |
|---|---|---|---|
| **Total Cost** | **~$780** | **~$2,000 – $2,200** | **~$4,000 – $6,000+** |
| **Memory Bandwidth** | **936 GB/s** | 273 GB/s | Up to ~800+ GB/s |
| **Whisper (1h Audio)** | **~25 – 35 sec** | ~55 – 70 sec | ~20 – 30 sec |
| **LLM Speed (70B 4-bit)** | **~18 – 22 tokens/sec** | ~8 – 10 tokens/sec | ~25 – 30 tokens/sec |
| **Giant Models (>24GB)** | ❌ Spills to slow CPU RAM | ✅ Fits up to ~50GB VRAM | ✅ **Runs 405B & 671B MoE** |
| **Training & Fine-Tuning** | ✅ **Full CUDA / Unsloth** | ⚠️ Limited MLX | ⚠️ Limited MLX |
| **Power Under AI Load** | ~350W peak | **~65W peak** | ~150W peak |

### Key Insight:
* **NVIDIA RTX 3090 on `abacus` ($780)** is ~2x faster than a $2,000 Mac mini on Whisper and LLMs up to 32B/70B, with full PyTorch/CUDA training support.
* **Apple Silicon (128GB–512GB)** is the premier platform for loading **extreme giant frontier models (405B dense / 671B DeepSeek-R1 MoE)** on a single desktop without building a $25,000 multi-GPU server rack.

---

## 7. eBay Used RTX 3090 Purchasing & Verification Guide

### Step 1: Search & Filter Protocol
1. **Condition**: Filter by **"Used"** or **"Certified Refurbished"** (never "For parts").
2. **Price Range**: **$600 to $750**. *(Listings under $450 are almost always compromised accounts/scams)*.
3. **Seller Reputation**: **$\ge$98.5% positive feedback**, account age $\ge$1–2 years, history of selling tech/PC components.
4. **Photos**: Real photos of the actual card, PCIe teeth, serial number sticker, and backplate (no stock photos).
5. **Description**: Look for explicit statements: *"100% working, fully tested in benchmarks/games."*

### Step 2: Recommended Partner Card Models
* **Tier 1 (Best Cooling for Rear VRAM)**:
  * **EVGA GeForce RTX 3090 FTW3 Ultra / XC3**
  * **ASUS TUF Gaming RTX 3090 24GB**
  * **ASUS ROG Strix RTX 3090 24GB**
* **Tier 2 (Solid Performance)**:
  * **MSI Gaming X Trio / Suprim X**
  * **NVIDIA Founders Edition (FE)**
  * **Gigabyte Gaming OC**
* **Tier 3 (Budget OEM)**:
  * Dell / HP Alienware OEM pulls (only consider if priced $\le$ $550).

### Step 3: Payment & Protection
* Always pay directly through eBay with **Credit Card** or **PayPal** to retain the 30-day **eBay Money Back Guarantee** (overrides seller "No Returns" policy if defective).

### Step 4: Arrival Verification Protocol
Upon delivery, run these tests before the 30-day window closes:
```bash
# 1. Verify card identity and full 24GB VRAM in Linux:
nvidia-smi

# 2. Check temperature under load (Whisper / Ollama):
# - GPU Core Target: <= 75°C
# - Memory Junction Target: <= 95°C - 100°C
watch -n 1 nvidia-smi --query-gpu=temperature.gpu,power.draw,utilization.gpu,memory.used --format=csv
```
If physical damage or excessive throttling occurs, open an eBay dispute for **"Item not as described"** for an automatic full refund.
