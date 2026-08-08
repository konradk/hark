import QtQuick

Canvas {
    id: icon

    property string symbol: ""
    property color color: "#d7dce5"

    width: 16
    height: 16
    onPaint: {
        const ctx = getContext("2d");
        ctx.reset();
        ctx.strokeStyle = color;
        ctx.fillStyle = color;
        ctx.lineWidth = 1.8;
        ctx.lineCap = "round";
        ctx.lineJoin = "round";
        if (symbol === "close") {
            ctx.beginPath();
            ctx.moveTo(4.5, 4.5);
            ctx.lineTo(11.5, 11.5);
            ctx.moveTo(11.5, 4.5);
            ctx.lineTo(4.5, 11.5);
            ctx.stroke();
        } else if (symbol === "refresh") {
            ctx.beginPath();
            ctx.arc(8, 8, 4.4, 0.55, 5.35, false);
            ctx.stroke();
            ctx.beginPath();
            ctx.moveTo(11.1, 2.9);
            ctx.lineTo(13.5, 3.2);
            ctx.lineTo(12.7, 5.4);
            ctx.stroke();
        } else if (symbol === "copy") {
            ctx.strokeRect(5.5, 4.5, 7, 8);
            ctx.strokeRect(3.5, 2.5, 7, 8);
        } else if (symbol === "chevron-down") {
            ctx.beginPath();
            ctx.moveTo(4.5, 6);
            ctx.lineTo(8, 9.5);
            ctx.lineTo(11.5, 6);
            ctx.stroke();
        } else if (symbol === "plus") {
            ctx.beginPath();
            ctx.moveTo(8, 3.8);
            ctx.lineTo(8, 12.2);
            ctx.moveTo(3.8, 8);
            ctx.lineTo(12.2, 8);
            ctx.stroke();
        } else if (symbol === "send") {
            ctx.beginPath();
            ctx.moveTo(3, 3.5);
            ctx.lineTo(13.5, 8);
            ctx.lineTo(3, 12.5);
            ctx.closePath();
            ctx.stroke();
            ctx.beginPath();
            ctx.moveTo(4.2, 8);
            ctx.lineTo(9, 8);
            ctx.stroke();
        } else if (symbol === "stop") {
            ctx.fillRect(4, 4, 8, 8);
        } else if (symbol === "paste") {
            ctx.strokeRect(4, 4.5, 8, 10);
            ctx.beginPath();
            ctx.moveTo(6, 3);
            ctx.lineTo(10, 3);
            ctx.lineTo(10.8, 5.5);
            ctx.lineTo(5.2, 5.5);
            ctx.closePath();
            ctx.stroke();
        } else if (symbol === "paperclip") {
            ctx.beginPath();
            ctx.moveTo(5, 8.5);
            ctx.lineTo(9.4, 4.1);
            ctx.bezierCurveTo(10.8, 2.7, 13.2, 4.1, 11.7, 5.7);
            ctx.lineTo(6.4, 11);
            ctx.bezierCurveTo(4.4, 13, 1.8, 10.4, 3.8, 8.4);
            ctx.lineTo(8.8, 3.4);
            ctx.stroke();
        } else if (symbol === "history") {
            ctx.beginPath();
            ctx.arc(8, 8, 5, -0.2, 5.15, false);
            ctx.stroke();
            ctx.beginPath();
            ctx.moveTo(3.1, 5.6);
            ctx.lineTo(3, 2.8);
            ctx.lineTo(5.4, 4.2);
            ctx.stroke();
            ctx.beginPath();
            ctx.moveTo(8, 5);
            ctx.lineTo(8, 8.2);
            ctx.lineTo(10.4, 9.7);
            ctx.stroke();
        } else if (symbol === "sliders") {
            ctx.beginPath();
            ctx.moveTo(3, 4.7);
            ctx.lineTo(13, 4.7);
            ctx.moveTo(3, 8);
            ctx.lineTo(13, 8);
            ctx.moveTo(3, 11.3);
            ctx.lineTo(13, 11.3);
            ctx.stroke();
            ctx.beginPath();
            ctx.arc(10.2, 4.7, 1.7, 0, Math.PI * 2);
            ctx.fill();
            ctx.beginPath();
            ctx.arc(5.8, 8, 1.7, 0, Math.PI * 2);
            ctx.fill();
            ctx.beginPath();
            ctx.arc(9.2, 11.3, 1.7, 0, Math.PI * 2);
            ctx.fill();
        } else if (symbol === "key") {
            ctx.beginPath();
            ctx.arc(5.8, 7.4, 2.4, 0, Math.PI * 2);
            ctx.stroke();
            ctx.beginPath();
            ctx.moveTo(8, 9);
            ctx.lineTo(13, 14);
            ctx.moveTo(10.3, 11.3);
            ctx.lineTo(11.7, 10);
            ctx.moveTo(11.7, 12.7);
            ctx.lineTo(13, 11.4);
            ctx.stroke();
        }
    }
    onColorChanged: requestPaint()
    onSymbolChanged: requestPaint()
}
