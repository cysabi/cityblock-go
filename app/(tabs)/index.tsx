import { useBackgroundLocation } from "@/hooks/use-location";
import { Camera, MapView } from "@maplibre/maplibre-react-native";
import { useEffect, useRef, useState } from "react";
import { ActivityIndicator, Text } from "react-native";

export default function HomeScreen() {
  const { location, status, error } = useBackgroundLocation();

  const mapRef: any = useRef(null);

  const [coords, setCoords] = useState<[number, number]>([
    -73.985056, 40.691327,
  ]);

  useEffect(() => {
    if (location) {
      setCoords([location.coords.longitude, location.coords.latitude]);
    }
  }, [location]);

  if (status === "starting") return <ActivityIndicator style={{ flex: 1 }} />;
  if (status === "error") return <Text>{error}</Text>;

  return (
    <MapView
      ref={mapRef}
      style={{ flex: 1 }}
      mapStyle="https://api.maptiler.com/maps/streets-v2/style.json?key=ZkJOL4BGmS6lWcFXLlfG"
    >
      <Camera centerCoordinate={coords} zoomLevel={17} />
    </MapView>
  );
}
