import { View, ActivityIndicator, Text } from "react-native";
import MapView, { Marker } from "react-native-maps";
import { useBackgroundLocation } from "@/hooks/use-location";

export default function HomeScreen() {
  const { location, status, error } = useBackgroundLocation();

  if (status === "starting") return <ActivityIndicator style={{ flex: 1 }} />;
  if (status === "error") return <Text>{error}</Text>;
  if (!location) return null;

  const { latitude, longitude } = location.coords;

  return (
    <View style={{ flex: 1 }}>
      <MapView
        style={{
          width: "100%",
          height: "100%",
        }}
        initialRegion={{
          latitude,
          longitude,
          latitudeDelta: 0.01,
          longitudeDelta: 0.01,
        }}
      >
        <Marker
          coordinate={{ latitude, longitude }}
          title={`last updated ${location.timestamp}`}
        />
      </MapView>
    </View>
  );
}
