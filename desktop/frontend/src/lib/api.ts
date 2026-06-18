// Thin typed wrapper over the Wails-generated Go bindings.
import {
  GetSnapshot,
  SetVolume,
  SetMuted,
  SetSolo,
  SetMasterVolume,
  SetLatencyProfile,
  Rename,
  ListOutputDevices,
  SetOutputDevice,
  Start,
  Stop,
} from "../../wailsjs/go/main/App";
import { engine } from "../../wailsjs/go/models";

export type Snapshot = engine.Snapshot;
export type Source = engine.Source;
export type OutputDevice = engine.OutputDevice;

export const api = {
  GetSnapshot,
  SetVolume,
  SetMuted,
  SetSolo,
  SetMasterVolume,
  SetLatencyProfile,
  Rename,
  ListOutputDevices,
  SetOutputDevice,
  Start,
  Stop,
};
