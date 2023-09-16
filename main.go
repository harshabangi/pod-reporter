package main

import (
	"context"
	"fmt"
	"github.com/carlmjohnson/requests"
	"github.com/labstack/echo/v4"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"strings"
	"time"
)

type statusContent struct {
	StageName         string `json:"stage_name"`
	StartTime         string `json:"start_time"`
	Duration          string `json:"duration"`
	IsCompleted       bool   `json:"is_completed"`
	InputRecordCount  int64  `json:"input_record_count"`
	OutputRecordCount int64  `json:"output_record_count"`
	TotalTasks        int    `json:"total_tasks"`
	CompletedTasks    int    `json:"completed_tasks"`
	InProgressTasks   int    `json:"in_progress_tasks"`
	ETA               string `json:"eta"`
}

func getStatusContent(c echo.Context) error {
	namespace := c.Param("namespace")
	podName := c.Param("pod_name")

	cl, err := NewInClusterKubernetesClient()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create Kubernetes client: "+err.Error())
	}

	pod, err := cl.CoreV1().Pods(namespace).Get(context.TODO(), podName, meta.GetOptions{})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get pod information: "+err.Error())
	}

	if pod.Status.Phase != corev1.PodRunning {
		return echo.NewHTTPError(http.StatusPreconditionFailed, "Pod is not in the 'Running' state")
	}

	return fetchPodStatus(pod.Status.PodIP, c.Request().Header.Get("Accept"), c)
}

func getStatusContentByLabels(c echo.Context) error {
	namespace := c.Param("namespace")
	labels := c.QueryParam("labels")

	labelSelector, err := labelsToSelector(labels)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid labels format: "+err.Error())
	}

	kubeClient, err := NewInClusterKubernetesClient()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create Kubernetes client: "+err.Error())
	}

	podList, err := kubeClient.CoreV1().Pods(namespace).List(context.TODO(), meta.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list pods: "+err.Error())
	}

	if len(podList.Items) != 1 {
		return echo.NewHTTPError(http.StatusNotFound, "No matching pods found or multiple pods matched")
	}

	pod := &podList.Items[0]
	if pod.Status.Phase != corev1.PodRunning {
		return echo.NewHTTPError(http.StatusPreconditionFailed, "Pod is not in the 'Running' state")
	}

	return fetchPodStatus(pod.Status.PodIP, c.Request().Header.Get("Accept"), c)
}

func labelsToSelector(labels string) (string, error) {
	labelPairs := strings.Split(labels, ",")

	var selectorParts []string
	for _, pair := range labelPairs {
		parts := strings.Split(pair, "=")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid label format: %s", pair)
		}
		key, value := parts[0], parts[1]
		selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", key, value))
	}

	return strings.Join(selectorParts, ","), nil
}

func deriveAcceptHeader(acceptHeader string) (string, error) {
	if acceptHeader == "" {
		acceptHeader = "text/html"
	}

	supportedMediaTypes := []string{"text/html", "application/json"}
	supported := false
	for _, mt := range supportedMediaTypes {
		if strings.Contains(acceptHeader, mt) {
			acceptHeader = mt
			supported = true
			break
		}
	}
	if !supported {
		return "", echo.NewHTTPError(http.StatusNotAcceptable, "Unsupported Accept header: "+acceptHeader)
	}
	return acceptHeader, nil
}

func fetchPodStatus(podIP, acceptHeader string, c echo.Context) error {
	url := fmt.Sprintf("http://%s:8081/status", podIP)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	acceptHeader, err := deriveAcceptHeader(acceptHeader)
	if err != nil {
		return err
	}

	switch acceptHeader {
	case "application/json":
		return fetchAndRespondJSON(c, url, ctx)
	case "text/html":
		return fetchAndRespondHTML(c, url, ctx)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "Unsupported Accept header: "+acceptHeader)
	}
}

func fetchAndRespondJSON(c echo.Context, url string, ctx context.Context) error {
	var response []statusContent
	err := requests.
		URL(url).
		Accept("application/json").
		ToJSON(&response).
		Fetch(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch status information: "+err.Error())
	}
	c.Response().Header().Set("Content-Type", "application/json")
	return c.JSON(http.StatusOK, response)
}

func fetchAndRespondHTML(c echo.Context, url string, ctx context.Context) error {
	var htmlContent string
	err := requests.
		URL(url).
		Accept("text/html").
		ToString(&htmlContent).
		Fetch(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch status information: "+err.Error())
	}
	c.Response().Header().Set("Content-Type", "text/html")
	return c.HTML(http.StatusOK, htmlContent)
}

func NewInClusterKubernetesClient() (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func main() {
	e := echo.New()
	e.GET("/v1/namespaces/:namespace/pods/:pod_name/status", getStatusContent)
	e.GET("/v1/namespaces/:namespace/pods/status", getStatusContentByLabels)
	e.Logger.Fatal(e.Start(":8080"))
}
